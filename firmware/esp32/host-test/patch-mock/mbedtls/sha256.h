#pragma once
// Native Windows test implementation of the same SHA-256 primitive.
#include <windows.h>
#include <bcrypt.h>
inline int mbedtls_sha256_ret(const unsigned char *data,size_t size,unsigned char*out,int){
 BCRYPT_ALG_HANDLE alg=nullptr;BCRYPT_HASH_HANDLE hash=nullptr;
 if(BCryptOpenAlgorithmProvider(&alg,BCRYPT_SHA256_ALGORITHM,nullptr,0))return -1;
 int result=BCryptCreateHash(alg,&hash,nullptr,0,nullptr,0,0)||BCryptHashData(hash,(PUCHAR)data,(ULONG)size,0)||BCryptFinishHash(hash,out,32,0);
 if(hash)BCryptDestroyHash(hash);BCryptCloseAlgorithmProvider(alg,0);return result;
}
