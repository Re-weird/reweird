package productdata

import (
	"context"
	"errors"
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ResolveUser upserts-on-first-sight: a duplicate (authProvider, authSubject)
// never creates a second user document, and an already-populated profile
// field is never overwritten by a fresher hint (only an empty field is
// filled in). See domain.UserProfileHints for why hints are treated this
// way.
func (store *MongoStore) ResolveUser(ctx context.Context, authProvider, authSubject string, hints domain.UserProfileHints) (domain.User, error) {
	if authProvider == "" || authSubject == "" {
		return domain.User{}, fmt.Errorf("productdata: authProvider and authSubject are required")
	}
	filter := bson.D{{Key: "auth_provider", Value: authProvider}, {Key: "auth_subject", Value: authSubject}}

	var existing domain.User
	err := store.users().FindOne(ctx, filter).Decode(&existing)
	switch {
	case err == nil:
		return store.fillProfileHints(ctx, filter, existing, hints)
	case errors.Is(err, mongo.ErrNoDocuments):
		return store.insertUser(ctx, filter, authProvider, authSubject, hints)
	default:
		return domain.User{}, fmt.Errorf("productdata: find user: %w", err)
	}
}

func (store *MongoStore) insertUser(ctx context.Context, filter bson.D, authProvider, authSubject string, hints domain.UserProfileHints) (domain.User, error) {
	id, err := newRandomID("user")
	if err != nil {
		return domain.User{}, err
	}
	now := nowMS()
	user := domain.User{
		ID:           id,
		AuthProvider: authProvider,
		AuthSubject:  authSubject,
		Email:        hints.Email,
		DisplayName:  hints.DisplayName,
		AvatarURL:    hints.AvatarURL,
		CreatedAtMS:  now,
		UpdatedAtMS:  now,
	}
	if _, err := store.users().InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Lost a race with a concurrent first-sight resolve for the same
			// identity; the other request's document is authoritative.
			var raced domain.User
			if fetchErr := store.users().FindOne(ctx, filter).Decode(&raced); fetchErr == nil {
				return raced, nil
			}
		}
		return domain.User{}, fmt.Errorf("productdata: insert user: %w", err)
	}
	return user, nil
}

func (store *MongoStore) fillProfileHints(ctx context.Context, filter bson.D, existing domain.User, hints domain.UserProfileHints) (domain.User, error) {
	set := bson.M{}
	if existing.Email == "" && hints.Email != "" {
		set["email"] = hints.Email
		existing.Email = hints.Email
	}
	if existing.DisplayName == "" && hints.DisplayName != "" {
		set["display_name"] = hints.DisplayName
		existing.DisplayName = hints.DisplayName
	}
	if existing.AvatarURL == "" && hints.AvatarURL != "" {
		set["avatar_url"] = hints.AvatarURL
		existing.AvatarURL = hints.AvatarURL
	}
	if len(set) == 0 {
		return existing, nil
	}
	now := nowMS()
	set["updated_at_ms"] = now
	existing.UpdatedAtMS = now
	if _, err := store.users().UpdateOne(ctx, filter, bson.M{"$set": set}); err != nil {
		return domain.User{}, fmt.Errorf("productdata: fill user profile hints: %w", err)
	}
	return existing, nil
}

func (store *MongoStore) GetUser(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := store.users().FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("productdata: get user: %w", err)
	}
	return &user, nil
}
