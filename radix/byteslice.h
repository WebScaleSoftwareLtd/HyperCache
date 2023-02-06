#ifdef __cplusplus
extern "C" {
#endif
#include <stdint.h>
#include <stddef.h>

#ifndef BYTESLICE_H
#define BYTESLICE_H
// Defines a slice which contains the length.
typedef struct {
    // Defines the value.
    uint8_t* value;

    // Defines the length.
    size_t length;
} ByteSlice;

ByteSlice* copy_byte_slice_heap(ByteSlice* original);
ByteSlice copy_byte_slice_stack(ByteSlice original);
#endif
#ifdef __cplusplus
}
#endif
