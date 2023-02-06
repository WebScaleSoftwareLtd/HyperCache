#ifdef __cplusplus
extern "C" {
#endif

#ifndef HYPERCACHE_ENDIANNESS_H
#define HYPERCACHE_ENDIANNESS_H

// TODO: Big endian support.

#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
    uint8_t* le_uint64_encode(uint64_t val) {
        auto ptr = (uint64_t*)&val;
        auto a = malloc(sizeof(uint64_t));
        memcpy(a, ptr, sizeof(uint64_t));
        return (uint8_t*)a;
    }

    uint64_t le_uint64_decode(const uint8_t* ptr) {
        return *(uint64_t*)ptr;
    }
#endif

#endif

#ifdef __cplusplus
}
#endif
