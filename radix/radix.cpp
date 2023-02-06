#ifndef _RADIX_CPP
#define _RADIX_CPP
#include "radix.hpp"
#include <shared_mutex>
#include <cinttypes>
#include <deque>
#include <utility>
#include <fstream>
#include "byteslice.h"
#include "endianness.h"

// Frees a nodes children. The amount of children killed will be the result.
inline size_t free_node_children(RadixTreeNode** nodes, size_t nodes_len) {
    // Defines the number of killed children.
    size_t killed_children{};

    // Get the nodes information stack.
    struct _node_information {
        size_t index;
        size_t len;
        RadixTreeNode** nodes;
    };
    auto info_stack = std::deque<_node_information*>();
    info_stack.push_back(new _node_information{
        .index = 0,
        .len = nodes_len,
        .nodes = nodes,
    });

    // Go through the nodes.
    while (!info_stack.empty()) {
        // Get the back info element.
        auto info = info_stack.back();

        // Make a stack copy.
        auto stack_cpy = *info;

        // If the length is equal to the index, we should pop and free this.
        if (stack_cpy.index == stack_cpy.len) {
            if (stack_cpy.len != 0) free(stack_cpy.nodes);
            info_stack.pop_back();
            delete info;
            continue;
        }

        // Get the next item.
        auto next = stack_cpy.nodes[stack_cpy.index];

        // Get the children and then free it.
        auto children = next->children;
        auto children_len = next->children_len;
        free(next->key.value);
        auto content = next->content;
        if (content) {
            free(content->value);
            free(content);
        }
        free(next);

        // Add 1 to the children killed.
        killed_children++;

        // Add 1 to the index.
        info->index = stack_cpy.index + 1;

        // Push the children to the stack.
        info_stack.push_back(new _node_information{
            .index = 0,
            .len = children_len,
            .nodes = children,
        });
    }

    // Return how many children were killed.
    return killed_children;
}

// Merges a child radix branch into a parent.
inline void merge_radix_branches(RadixTreeNode* parent, RadixTreeNode* child) {
    // Concat the key and GC both the key of the other child and last parent.
    auto key_len = child->key.length + parent->key.length;
    auto concat = (uint8_t*)malloc(key_len);
    memcpy(concat, parent->key.value, parent->key.length);
    free(parent->key.value);
    memcpy(&concat[parent->key.length], child->key.value, child->key.length);
    free(child->key.value);

    // Free the children array.
    free(parent->children);

    // Copy everything from the other child.
    parent->children = child->children;
    parent->children_len = child->children_len;
    parent->content = child->content;

    // Copy over the key.
    parent->key.length = key_len;
    parent->key.value = concat;
}

RadixTreeBranchWalker::RadixTreeBranchWalker(RadixTreeNode* node, ByteSlice full_key, size_t key_chunk, std::shared_mutex* lock) {
    // Handle a null node.
    if (!node) {
        unlocked = true;
        return;
    }

    // Insert the lock.
    this->lock = lock;

    // Create a chunk of the key.
    auto* a = (uint8_t*)malloc(key_chunk);
    if (key_chunk > full_key.length) {
        // Copy all the parts involved.
        memcpy(a, full_key.value, full_key.length);
        auto remainder_len = key_chunk - full_key.length;
        memcpy(&a[full_key.length], &node->key.value[remainder_len], remainder_len);
    } else {
        // Copy the key.
        memcpy(a, full_key.value, key_chunk);
    }
    ByteSlice s{};
    s.value = a;
    s.length = key_chunk;

    // Add the child.
    add_node_child(node, s);
}

// Walk through the radix tree. A null pointer means you have reached the end.
RadixTreeWalkValue* RadixTreeBranchWalker::next() {
    for (;;) {
        // If the nodes count is 0, unlock the radix tree and return null.
        if (nodes.empty()) {
            if (!unlocked) {
                lock->unlock_shared();
                unlocked = true;
            }
            return nullptr;
        }

        // Get the last child.
        RadixTreeNode* last_child = nodes.back();

        // If the childs value isn't read, we will do that.
        if (!childs_value_read) {
            // We're about to read it, we don't want to worry about this child again.
            childs_value_read = true;

            // Check if there is content.
            if (last_child->content) {
                // There is! Return this now.
                auto key = get_current_key();
                auto value = copy_byte_slice_stack(*last_child->content);
                auto v = (RadixTreeWalkValue*)malloc(sizeof(RadixTreeWalkValue));
                v->key = key;
                v->value = value;
                return v;
            }
        }

        // Get the child index.
        auto child_index = child_indexes.back();

        // If the child index is equal to the child count, we're done with this node.
        if (child_index == last_child->children_len) {
            remove_node_child();
            continue;
        }

        // Get the nodes child.
        add_node_child(last_child->children[child_index], last_child->key);
        childs_value_read = false;
    }
}

void RadixTreeBranchWalker::add_node_child(RadixTreeNode* node, ByteSlice key_chunk) {
    nodes.push_back(node);
    key_chunks.push_back(key_chunk);
    child_indexes.push_back(0);
}

void RadixTreeBranchWalker::remove_node_child() {
    nodes.pop_back();
    key_chunks.pop_back();
    child_indexes.pop_back();
    if (!child_indexes.empty()) {
        // Since we are done with this child, we want to add 1 to the parent.
        auto back = child_indexes.back();
        back++;
        child_indexes.pop_back();
        child_indexes.push_back(back);
    }
}

ByteSlice RadixTreeBranchWalker::get_current_key() {
    size_t len{};
    auto iter = nodes.begin();
    for (;;) {
        // If iter is false, break.
        if (!*iter) break;

        // Get the key length.
        len += (*iter)->key.length;

        // Get the next value.
        iter++;
    }
    auto* data = (uint8_t*)malloc(len);
    size_t done{};
    iter = nodes.begin();
    for (;;) {
        // If iter is false, break.
        if (!*iter) break;

        // Get the key.
        auto key = (*iter)->key;

        // Copy the memory.
        memcpy(&data[done], key.value, key.length);

        // Add to done.
        done += key.length;

        // Get the next value.
        iter++;
    }
    ByteSlice s{};
    s.length = len;
    s.value = data;
    return s;
}

RadixTreeRoot::RadixTreeRoot() {
    node = (RadixTreeNode*)calloc(1, sizeof(RadixTreeNode));
}

RadixTreeRoot::RadixTreeRoot(RadixTreeNode** nodes, size_t nodes_len) {
    node = (RadixTreeNode*)calloc(1, sizeof(RadixTreeNode));
    node->children = nodes;
    node->children_len = nodes_len;
}

void RadixTreeRoot::free_tree() {
    lock.lock();
    auto children = node->children;
    auto children_len = node->children_len;
    auto content = node->content;
    if (content) {
        free(content->value);
        free(content);
    }
    free(node);
    node = (RadixTreeNode*)calloc(1, sizeof(RadixTreeNode));
    lock.unlock();
    free_node_children(children, children_len);
}

ByteSlice* RadixTreeRoot::get(ByteSlice key) {
    // Acquire the shared mutex lock.
    lock.lock_shared();

    // Get the node.
    RadixTreeNodeResult result = un_thread_safe_get_node(key, false);
    if (result.key_index != key.length) {
        // Not a strict match!
        lock.unlock_shared();
        return nullptr;
    }

    // Copy the value and return.
    auto* cpy = copy_byte_slice_heap(result.node->content);
    lock.unlock_shared();
    return cpy;
}

// Walk items starting with a prefix.
RadixTreeBranchWalker RadixTreeRoot::walk_prefix(ByteSlice key) {
    // Acquire the shared mutex lock. This is unlocked by the walker.
    lock.lock_shared();

    // Get the node.
    auto result = un_thread_safe_get_node(key, true);
    if (key.length > result.key_index) {
        // Null node.
        lock.unlock_shared();
        return RadixTreeBranchWalker(nullptr, key, 0, nullptr);
    }
    return RadixTreeBranchWalker(result.node, key, result.key_index, &lock);
}

// Split a node at index. The remainder after the index will become its own child,
// and the parent will essentially just become a router. The split node is returned.
RadixTreeNode* split_node(size_t split_index, RadixTreeNode* node, RadixTreeNode* other_child) {
    // Get the initial node key so we don't have to keep accessing the pointer.
    const auto init_node_key = node->key;

    // Allocate for the key pre-split.
    auto key_pre_split = (uint8_t*)malloc(split_index);
    memcpy(key_pre_split, init_node_key.value, split_index);

    // Get the length of the remainder.
    auto remainder_len = node->key.length - split_index;

    // Allocate for the remainder.
    auto key_remainder = (uint8_t*)malloc(remainder_len);
    memcpy(key_remainder, &init_node_key.value[split_index], remainder_len);

    // Create a new child for the current contents of the node.
    auto new_child = (RadixTreeNode*)malloc(sizeof(RadixTreeNode));
    memcpy(new_child, node, sizeof(RadixTreeNode));

    // Set the key of the new child to the remainder.
    new_child->key.value = key_remainder;
    new_child->key.length = remainder_len;

    // Null the content of the node since this basically just works as a router now.
    node->content = nullptr;

    // Allocate for a place for the 1 or 2 children.
    auto children_len = other_child ? 2 : 1;
    auto children = (RadixTreeNode**)malloc(children_len * sizeof(RadixTreeNode*)); // NOLINT

    // Add the node split as a child.
    *children = new_child;

    // If there's another child, we'll add it.
    if (other_child) children[1] = other_child;

    // Set the nodes children.
    node->children = children;
    node->children_len = children_len;

    // Set the nodes key to the common bit.
    node->key.length = split_index;
    node->key.value = key_pre_split;

    // Return the new child.
    return new_child;
}

// Sets a keys value in the tree. Returns true if it overwrote something.
// The value will not be copied, but the key will. Free your key, do NOT free your value!
bool RadixTreeRoot::set(ByteSlice key, ByteSlice value) {
    // Acquire the write lock.
    lock.lock();

    // Get as close to the node as possible.
    auto result = un_thread_safe_get_node(key, false);
    if (result.key_index == key.length) {
        // It is a strict match. Is this an overwrite?
        if (result.node->content) {
            // Overwrite the contents.
            result.node->content->length = value.length;
            result.node->content->value = value.value;
            lock.unlock();
            return true;
        }

        // Set the contents. This is duplicated from below since if we did it any earlier it'd leak on overwrite.
        auto value_heap = (ByteSlice*)malloc(sizeof(ByteSlice));
        value_heap->value = value.value;
        value_heap->length = value.length;
        result.node->content = value_heap;

        // Return false since this had no contents, making it just a router at the time.
        lock.unlock();
        return false;
    }

    // Defines a heap allocation of the value.
    auto value_heap = (ByteSlice*)malloc(sizeof(ByteSlice));
    value_heap->value = value.value;
    value_heap->length = value.length;

    // Find if there is any keys we can break down.
    for (size_t i = 0; i < result.node->children_len; i++) {
        // Get the child.
        auto child = result.node->children[i];

        // Check if it starts with anything from the start of key index.
        auto child_key = child->key;
        uintptr_t x = 0;
        auto y = result.key_index;
        for (; y < key.length; y++) {
            if (x == child_key.length) {
                // We don't want a buffer overflow.
                break;
            }
            if (key.value[y] != child_key.value[x]) {
                // We are done here with possible optimisations.
                break;
            }
            x++;
        }

        // Check if there is anything in common with this key.
        auto common = y - result.key_index;
        if (common != 0) {
            // Defines the other child. If the bit in common isn't the key, we'll store our content here.
            RadixTreeNode* other_child{};
            if (common != key.length) {
                // The key isn't the bit in common, this means there'll be a new child for the contents.
                // Allocate the child here.
                other_child = (RadixTreeNode*)malloc(sizeof(RadixTreeNode));
                other_child->children_len = 0;
                auto other_child_key = ByteSlice{};
                other_child_key.length = key.length - common;
                other_child_key.value = (uint8_t*)malloc(other_child_key.length);
                memcpy(other_child_key.value, &key.value[common], other_child_key.length);
                other_child->key = other_child_key;
                other_child->content = value_heap;
            }

            // Split the node.
            split_node(common, child, other_child);
            if (common == key.length) {
                // Since it is common, we want to set the content here.
                child->content = value_heap;
            }

            // Unlock the mutex.
            lock.unlock();

            // Return false since we did not overwrite anything.
            return false;
        }
    }

    // We were not able to optimise any further. Just add to where we are.
    auto** children = (RadixTreeNode**)malloc(sizeof(RadixTreeNode*) * (result.node->children_len + 1)); // NOLINT
    for (int i = 0; i < result.node->children_len; i++) children[i] = result.node->children[i];
    auto branch_entry = &children[result.node->children_len];
    if (result.node->children_len != 0) {
        // This is definitely memory allocated, we will free it.
        free(result.node->children);
    }
    result.node->children_len++;
    result.node->children = children;

    // Create the remainder key.
    auto remainder_len = key.length - result.key_index;
    auto* remainder_chunk = (uint8_t*)malloc(remainder_len);
    memcpy(remainder_chunk, &key.value[result.key_index], remainder_len);

    // Create the child.
    auto* child = (RadixTreeNode*)calloc(1, sizeof(RadixTreeNode));
    ByteSlice key_chunk{};
    key_chunk.length = remainder_len;
    key_chunk.value = remainder_chunk;
    child->key = key_chunk;
    child->content = value_heap;
    *branch_entry = child;

    // Unlock and return false.
    lock.unlock();
    return false;
}

// Removes items from the tree by prefix.
size_t RadixTreeRoot::delete_prefix(ByteSlice key) {
    // Write lock the mutex.
    lock.lock();

    // If the keys length is zero, handle removing content from the base node.
    if (key.length == 0) {
        auto children = node->children;
        auto children_len = node->children_len;
        node->children_len = 0;
        lock.unlock();
        return free_node_children(children, children_len);
    }

    // Defines the current key index.
    size_t key_index = 0;

    // Defines the child.
    RadixTreeNode* child;

    // Defines the current node.
    RadixTreeNode* current_node = node;

    // Loop through the tree children.
    for (;;) {
        // Go through the nodes.
        bool outer_continue = false;
        for (size_t i = 0; i < current_node->children_len; i++) {
            // Get the child.
            child = current_node->children[i];

            // Check if the key is a chunk of ours.
            if (key.length >= key_index + child->key.length) {
                // Ok, it isn't too long. We can check without accidentally crashing or leaking memory.

                // Check if the key chunk is there.
                if (memcmp(&key.value[key_index], child->key.value, child->key.length) == 0) {
                    // Add to the key index.
                    key_index += child->key.length;

                    // Check if this is the key.
                    if (key_index == key.length) {
                        // We matched!
                        un_thread_safe_cut_branch(node, current_node, child);
                        lock.unlock();
                        auto children = child->children;
                        auto children_len = child->children_len;
                        free(child);
                        return 1 + free_node_children(children, children_len);
                    } else {
                        // Go back to the start.
                        current_node = child;
                        outer_continue = true;
                        break;
                    }
                }
            } else if (key_index + child->key.length > key.length) {
                // Check if it starts with anything from the start of key index.
                auto child_key = child->key;
                uintptr_t x = 0;
                auto y = key_index;
                for (; y < key.length; y++) {
                    if (x == child_key.length) {
                        // We don't want a buffer overflow.
                        break;
                    }
                    if (key.value[y] != child_key.value[x]) {
                        // We are done here with possible node.
                        break;
                    }
                    x++;
                }
                if (y == key.length) {
                    // This is it. We have exhausted the key.
                    // Cut the branch.
                    un_thread_safe_cut_branch(node, current_node, child);
                    lock.unlock();
                    auto children = child->children;
                    auto children_len = child->children_len;
                    free(child);
                    return 1 + free_node_children(children, children_len);
                }
            }
        }
        if (outer_continue) continue;

        // We didn't match.
        lock.unlock();
        return 0;
    }
}

// Removes an item from the tree. Returns true if an item is deleted.
bool RadixTreeRoot::delete_key(ByteSlice key) {
    // Write lock the mutex.
    lock.lock();

    // If the keys length is zero, handle removing content from the base node.
    if (key.length == 0) {
        bool exists = node->content;
        if (exists) {
            free(node->content->value);
            free(node->content);
            node->content = nullptr;
        }
        lock.unlock();
        return exists;
    }

    // Defines the current key index.
    size_t key_index = 0;

    // Defines the child.
    RadixTreeNode* child;

    // Defines the current node.
    RadixTreeNode* current_node = node;

    // Loop through the tree children.
    for (;;) {
        // Go through the nodes.
        bool outer_continue = false;
        for (size_t i = 0; i < current_node->children_len; i++) {
            // Get the child.
            child = current_node->children[i];

            // Check if the key is a chunk of ours.
            if (key.length >= key_index + child->key.length) {
                // Ok, it isn't too long. We can check without accidentally crashing or leaking memory.

                // Check if the key chunk is there.
                if (memcmp(&key.value[key_index], child->key.value, child->key.length) == 0) {
                    // Add to the key index.
                    key_index += child->key.length;

                    // Check if this is the key.
                    if (key_index == key.length) {
                        // We matched!
                        un_thread_safe_cut_branch(node, current_node, child);
                        lock.unlock();
                        return true;
                    } else {
                        // Go back to the start.
                        current_node = child;
                        outer_continue = true;
                        break;
                    }
                }
            }
        }
        if (outer_continue) continue;

        // We didn't match.
        lock.unlock();
        return false;
    }
}

// Cuts a branch from the tree.
void un_thread_safe_cut_branch(RadixTreeNode* root, RadixTreeNode* parent, RadixTreeNode* branch) {
    // Free our current node.
    if (branch->content) {
        // We are nuking this branches content.
        free(branch->content->value);
        free(branch->content);
        branch->content = nullptr;
    }

    // If the child length is 0, this node is definitely unused.
    if (branch->children_len == 0) {
        // This is a dead branch. Free it.
        free(branch->key.value);
        free(branch);

        // Check if there's just one other child in the parent and no content. If so, concat the child and return.
        if (parent->children_len == 2 && !parent->content && parent != root) {
            // Get the other child.
            auto other_child = *parent->children == branch ? parent->children[1] : *parent->children;

            // Merge the 2 branches.
            merge_radix_branches(parent, other_child);

            // Free the other child.
            free(other_child);

            // We have dealt with this and can return now.
            return;
        }

        // Reallocate the children array.
        parent->children_len--;
        auto nodes = (RadixTreeNode**)malloc(sizeof(RadixTreeNode*) * parent->children_len); // NOLINT
        size_t x = 0;
        for (auto i = 0; i < parent->children_len + 1; i++) {
            auto child_node = parent->children[i];
            if (child_node != branch) {
                nodes[x] = child_node;
                x++;
            }
        }
        free(parent->children);
        parent->children = nodes;
    }
}

// Gets a node from the tree.
RadixTreeNodeResult RadixTreeRoot::un_thread_safe_get_node(ByteSlice key, bool allow_node_prefix) {
    // If the keys length is zero, get the base node.
    if (key.length == 0) {
        RadixTreeNodeResult result{};
        result.key_index = 0;
        result.node = node;
        return result;
    }

    // Defines the current key index.
    size_t key_index = 0;

    // Defines the current node.
    RadixTreeNode* current_node = node;

    // Loop through the tree children.
    get_loop_start:
    // Check if this is the end of the key.
    if (key_index >= key.length) {
        // It is! We have the node here.
        RadixTreeNodeResult result{};
        result.key_index = key_index;
        result.node = current_node;
        return result;
    }

    // Look through the children.
    for (size_t i = 0; i < current_node->children_len; i++) {
        // Get the child.
        RadixTreeNode* child = current_node->children[i];

        // Check if the key is a chunk of ours.
        if (key.length >= key_index + child->key.length) {
            if (memcmp(&key.value[key_index], child->key.value, child->key.length) == 0) {
                // This would mean that the child is a chunk of the key.

                // Add to the key index.
                key_index += child->key.length;

                // Make this the node.
                current_node = child;

                // Go to the start of the loop.
                goto get_loop_start;
            }
        } else if (key_index + child->key.length > key.length && allow_node_prefix) {
            // Check if it starts with anything from the start of key index.
            auto child_key = child->key;
            uintptr_t x = 0;
            auto y = key_index;
            for (; y < key.length; y++) {
                if (x == child_key.length) {
                    // We don't want a buffer overflow.
                    break;
                }
                if (key.value[y] != child_key.value[x]) {
                    // We are done here with possible node.
                    break;
                }
                x++;
            }
            if (y == key.length) {
                // This is it. We have exhausted the key.
                // Enough of the child is used.

                // Add to the key index.
                key_index += child->key.length;

                // Make this the node.
                current_node = child;

                // Go to the start of the loop.
                goto get_loop_start;
            }
        }
    }

    // No luck.
    RadixTreeNodeResult result{};
    result.key_index = key_index;
    result.node = current_node;
    return result;
}

bool set_with_stack_value(RadixTreeRoot* tree, ByteSlice key, uint8_t* value_start, size_t value_len) {
    uint8_t* value_cpy = (uint8_t*)malloc(value_len);
    memcpy(value_cpy, value_start, value_len);
    auto value = ByteSlice{};
    value.value = value_cpy;
    value.length = value_len;
    return tree->set(key, value);
}
#endif // _RADIX_CPP
