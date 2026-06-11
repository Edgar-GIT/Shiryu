#include "curl_wrapper.h"
#if defined(__has_include)
#  if __has_include(<curl/curl.h>)
#    include <curl/curl.h>
#    define HAVE_LIBCURL 1
#  else
#    define HAVE_LIBCURL 0
#  endif
#else
#  define HAVE_LIBCURL 0
#endif
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

int curl_global_init_wrapper(void) {
#if HAVE_LIBCURL
    return curl_global_init(CURL_GLOBAL_ALL);
#else
    return 0;
#endif
}

void curl_global_cleanup_wrapper(void) {
#if HAVE_LIBCURL
    curl_global_cleanup();
#else
    (void)0;
#endif
}

static size_t write_cb(void *ptr, size_t size, size_t nmemb, void *userdata) {
    FILE *fp = (FILE*) userdata;
    if (!fp) return 0;
    return fwrite(ptr, size, nmemb, fp);
}

int curl_download_range(const char* url, const char* out_path, unsigned long long start, unsigned long long end, int resume, long long *bytes_received) {
#if HAVE_LIBCURL
    CURL *curl = curl_easy_init();
    if (!curl) return -1;
    CURLcode rc = CURLE_OK;

    FILE *fp = NULL;
    if (resume) fp = fopen(out_path, "ab");
    else fp = fopen(out_path, "wb");
    if (!fp) { curl_easy_cleanup(curl); return -2; }

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, fp);

    char range_header[128];
    if (end > 0) {
        snprintf(range_header, sizeof(range_header), "%llu-%llu", (unsigned long long)start, (unsigned long long)end);
    } else {
        snprintf(range_header, sizeof(range_header), "%llu-", (unsigned long long)start);
    }
    curl_easy_setopt(curl, CURLOPT_RANGE, range_header);

    curl_easy_setopt(curl, CURLOPT_FAILONERROR, 1L);
    curl_easy_setopt(curl, CURLOPT_NOPROGRESS, 1L);

    rc = curl_easy_perform(curl);

    if (bytes_received) {
        double dlnow = 0.0;
        if (curl_easy_getinfo(curl, CURLINFO_SIZE_DOWNLOAD, &dlnow) == CURLE_OK) {
            *bytes_received = (long long) dlnow;
        } else {
            *bytes_received = -1;
        }
    }

    fclose(fp);
    curl_easy_cleanup(curl);
    return (rc == CURLE_OK) ? 0 : (int)rc;
#else
    // Fallback: call system curl if libcurl is not available at compile time.
    // Build command: curl --fail -s --location --range START-END 'URL' >> 'OUTPATH'
    char cmd[1024];
    if (end > 0) {
        snprintf(cmd, sizeof(cmd), "curl --fail -s --location --range %llu-%llu '%s' >> '%s'", (unsigned long long)start, (unsigned long long)end, url, out_path);
    } else {
        snprintf(cmd, sizeof(cmd), "curl --fail -s --location --range %llu- '%s' >> '%s'", (unsigned long long)start, url, out_path);
    }
    int rc = system(cmd);
    if (bytes_received) *bytes_received = -1;
    return rc;
#endif
}
