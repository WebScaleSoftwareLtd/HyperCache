# Radix Binary Format

The goal of this format is to store and load radix trees created by HyperCache in the most efficient form possible. To do this, we use a solely binary format which is predictable.

## File Format
The file should start with the the header `RBF1` and then contain [children](#children-representation). The children will form the base node.

## Children Representation
Children start with a uint64 little endian number which represents how many children there are. From here, followed will be each [node](#node-representation).

## Node Representation

The node data starts with a uint64 little endian number for the key length. From here, the number of bytes specified in this number will contain the nodes key.

Following this is the nodes content. The content length will be a uint64 little endian and will be the first part of the content. From here, if the content length is 0, there is a byte to check if the content is null. If this byte is `0x01`, the content will be marked as null. Following this will be the number of bytes specified in the content length and will contain the content.

After this, the [children](#children-representation) will be present.
