#ifdef __cplusplus
extern "C" {
#endif

#include "byteslice.h"
#include <stdlib.h>
#include <string.h>

// Defines a function to copy the byte slice with a heap ByteSlice.
ByteSlice *copy_byte_slice_heap(ByteSlice *original) {
    // Null originals will always be null.
    if (!original) return NULL;

    // Create a copy.
    ByteSlice *cpy = (ByteSlice *) malloc(sizeof(ByteSlice));
    cpy->length = original->length;
    cpy->value = (uint8_t *) malloc(cpy->length);
    memcpy(cpy->value, original->value, cpy->length);
    return cpy;
}

// Defines a function to copy the byte slice with a stack ByteSlice.
ByteSlice copy_byte_slice_stack(ByteSlice original) {
    ByteSlice cpy;
    cpy.length = original.length;
    cpy.value = (uint8_t *) malloc(cpy.length);
    memcpy(cpy.value, original.value, cpy.length);
    return cpy;
}

#ifdef __cplusplus
}
#endif
