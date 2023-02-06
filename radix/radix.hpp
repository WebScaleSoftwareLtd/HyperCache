#ifndef RADIX_H
#define RADIX_H
#include <shared_mutex>
#include <cinttypes>
#include <deque>
#include "byteslice.h"

// Defines a node of the radix tree.
struct RadixTreeNode {
    // Defines the length of children.
    size_t children_len;

    // Defines a pointer to the children.
    RadixTreeNode** children;

    // Defines the key.
    ByteSlice key;

    // Defines the contents of this node.
    ByteSlice* content;
};

// Defines a node result.
struct RadixTreeNodeResult {
    // Used internally to check if it is a strict match.
    size_t key_index;

    // Defines the tree node.
    RadixTreeNode* node;
};

// Defines a walking value result.
struct RadixTreeWalkValue {
    // Defines the key.
    ByteSlice key;

    // Defines the value.
    ByteSlice value;
};

class RadixTreeBranchWalker {
    public:
        RadixTreeBranchWalker(RadixTreeNode* node, ByteSlice full_key, size_t key_chunk, std::shared_mutex* lock);
        RadixTreeWalkValue* next();
    private:
        void add_node_child(RadixTreeNode* node, ByteSlice key_chunk);
        void remove_node_child();
        ByteSlice get_current_key();

        // Defines if the childs value has been read.
        bool childs_value_read{};

        // Defines if the radix tree was unlocked.
        bool unlocked;

        // Defines the nodes in order of parents first.
        std::deque<RadixTreeNode*> nodes;
        std::deque<ByteSlice> key_chunks;
        std::deque<uintptr_t> child_indexes;

        // Defines the radix trees mutex.
        std::shared_mutex* lock;
};

class RadixTreeRoot {
    public:
        RadixTreeRoot();
        RadixTreeRoot(RadixTreeNode** nodes, size_t nodes_len);
        ByteSlice* get(ByteSlice key);
        RadixTreeBranchWalker walk_prefix(ByteSlice key);
        bool set(ByteSlice key, ByteSlice value);
        bool delete_key(ByteSlice key);
        size_t delete_prefix(ByteSlice key);
        void free_tree();
        RadixTreeNode* node;
#ifndef SWIG
        mutable std::shared_mutex lock;
#endif
    private:
#ifdef SWIG
        mutable std::shared_mutex lock;
#endif
        RadixTreeNodeResult un_thread_safe_get_node(ByteSlice key, bool allow_node_prefix);
};

inline size_t free_node_children(RadixTreeNode** nodes, size_t nodes_len);
inline void merge_radix_branches(RadixTreeNode* parent, RadixTreeNode* child);
void un_thread_safe_cut_branch(RadixTreeNode* root, RadixTreeNode* parent, RadixTreeNode* branch);
RadixTreeNode* split_node(size_t split_index, RadixTreeNode* node, RadixTreeNode* other_child);
bool set_with_stack_value(RadixTreeRoot* tree, ByteSlice key, uint8_t* value_start, size_t value_len);
#endif
