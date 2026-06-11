#ifndef CURL_WRAPPER_H
#define CURL_WRAPPER_H

#include <stddef.h>
#include <stdint.h>

int curl_global_init_wrapper(void);
void curl_global_cleanup_wrapper(void);

int curl_download_range(const char* url, const char* out_path, unsigned long long start, unsigned long long end, int resume, long long *bytes_received);

#endif
