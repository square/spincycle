package main

import (
	"errors"
	"fmt"
)

// - Ask to make cache with size 0 or size 1
// - Code comments
// - How can you refactory? Common functionality for "mark node as most recently used" in set and get
// - Write function to print everything out
// - Write tests.

var (
	ErrKeyNotFound = errors.New("key not found in cache")
)

type Node struct {
	Prev *Node
	Next *Node
	Key  string
	Val  string
}

type Cache struct {
	Head  *Node            // The oldest item in the cache.
	Tail  *Node            // The newest item in the cache.
	Nodes map[string]*Node // Map of node value => node
	Size  int              // The size of the cache.

	// @todo: add concurrency control
}

func NewCache(size int) (*Cache, error) {
	if size < 1 {
		return nil, fmt.Errorf("cache size must be greater than 1")
	}

	return &Cache{
		Size:  size,
		Nodes: map[string]*Node{},
	}, nil
}

func (c *Cache) Set(key, val string) {
	node := Node{
		Key: key,
		Val: val,
	}

	// If this is the first item in the cache, it is the head and tail.
	if len(c.Nodes) == 0 {
		c.Head = &node
		c.Tail = &node
		c.Nodes[key] = &node
	} else {
		// If the cache is full, evict the oldest node.
		if len(c.Nodes) == c.Size {
			c.Delete(c.Head.Key)
		}

		// Handle special case of when cache size is 1.
		if len(c.Nodes) == 0 {
			c.Head = &node
			c.Tail = &node
			c.Nodes[key] = &node
			return
		}

		// Add this node to the end of the cache.
		node.Next = c.Tail
		node.Next.Prev = &node
		c.Tail = &node
		c.Nodes[key] = &node
	}
}

// Delete deletes a key from the cache. It does not return anything regardless
// of whether or not the key exists.
func (c *Cache) Delete(key string) {
	fmt.Printf("nodes: %d, size: %d\n", len(c.Nodes), c.Size)
	if node, ok := c.Nodes[key]; ok {
		if len(c.Nodes) == 1 {
			c.Head = nil
			c.Tail = nil
			delete(c.Nodes, key)
		} else {
			if node == c.Head {
				c.Head = node.Prev
			} else if node == c.Tail {
				c.Tail = node.Next
			}
			delete(c.Nodes, key)
		}
	}
}

// Get gets the value of a key from the cache. It returns an error if the key
// does not exist.
func (c *Cache) Get(key string) (string, error) {
	if node, ok := c.Nodes[key]; ok {
		c.moveNodeToTail(node)
		return node.Val, nil
	} else {
		return "", ErrKeyNotFound
	}
}

// PrintAll prints all of the nodes in the cache to stdout.
func (c *Cache) PrintAll() {
	fmt.Println("Printing out items in cache in order from least recently " +
		"accessed to most recently accessed")
	for node := c.Head; node != nil; node = node.Prev {
		fmt.Printf("%s, %s\n", node.Key, node.Val)
	}
}

// ------------------------------------------------------------------------- //

func (c *Cache) moveNodeToTail(node *Node) {
	if node == c.Tail {
		// The node is already the Tail, do nothing.
		return
	} else {
		if node == c.Head {
			c.Head = node.Prev
			c.Head.Next = nil
		} else {
			node.Next.Prev = node.Prev
			node.Prev.Next = node.Next
		}

		// Move this node to the end of the cache.
		node.Next = c.Tail
		node.Prev = nil
		node.Next.Prev = node
		c.Tail = node
	}
}

func main() {
	c, err := NewCache(0)
	if err == nil {
		fmt.Println("expected an error but did not get one")
	}

	c, err = NewCache(5)
	if err != nil {
		fmt.Printf("error = %s, expected nil\n", err)
		return
	}

	c.Set("apple", "red")
	c.Set("pear", "green")
	val, err := c.Get("apple")
	if err != nil {
		fmt.Printf("error = %s, expected nil\n", err)
		return
	}
	if val != "red" {
		fmt.Printf("apple value = %s, expected %s\n", val, "red")
		return
	}

	c.Set("banana", "yellow")
	c.Set("peach", "orange")
	c.Set("grape", "purple")

	if len(c.Nodes) != 5 {
		fmt.Printf("%d items in the cache, expected %d\n", len(c.Nodes), 5)
		return
	}

	// This set should remove "pear" from the cache.
	c.Set("blueberry", "blue")

	if len(c.Nodes) != 5 {
		fmt.Printf("%d items in the cache, expected %d\n", len(c.Nodes), 5)
		return
	}

	c.PrintAll()
}
