package gola

import (
	"fmt"
	"sort"
	"strings"
)

type NodeType int

const (
	NodeTypeEndPoint NodeType = iota
	NodeTypeNamespace
	NodeTypeRecursive
	NodeTypeRoot
)

type Node interface {
	Parent() Node
	Handlers() []Handler
	Name() string
	ParameterName() string
	Children() map[string]Node
	NodeType() NodeType
}

type _Node struct {
	parent        Node
	name          string
	parameterName string
	handlers      []Handler
	children      map[string]Node
	nodeType      NodeType
}

func (n *_Node) path() string {
	rtn := ""
	var current Node = n
	if current.NodeType() == NodeTypeRoot {
		return "/"
	}

	for {
		switch current.NodeType() {
		case NodeTypeRoot:
			rtn = fmt.Sprintf("/%s", rtn)
		case NodeTypeEndPoint:
			rtn = fmt.Sprintf("%s/:%s/%s", current.Name(), current.ParameterName(), rtn)
		case NodeTypeRecursive:
			rtn = fmt.Sprintf("%s/%s*", current.Name(), rtn)
		case NodeTypeNamespace:
			rtn = fmt.Sprintf("%s/%s", current.Name(), rtn)
		}

		if current.Parent() == nil {
			break
		}

		current = current.Parent()
	}

	return strings.TrimRight(rtn, "/")
}

func (n *_Node) Parent() Node {
	return n.parent
}

func (n *_Node) Handlers() []Handler {
	return n.handlers
}

func (n *_Node) Name() string {
	return n.name
}

func (n *_Node) ParameterName() string {
	return n.parameterName
}

func (n *_Node) Children() map[string]Node {
	return n.children
}

func (n *_Node) NodeType() NodeType {
	return n.nodeType
}

type Route struct {
	root Node
}

func NewRoute() *Route {
	return &Route{root: &_Node{
		parent:   nil,
		name:     "",
		children: map[string]Node{},
		nodeType: NodeTypeRoot,
	}}
}

func (r *Route) traverse(node Node, result map[string]int) {
	if len(node.Children()) > 0 {
		for _, n := range node.Children() {
			r.traverse(n, result)
		}
	}

	switch node.NodeType() {
	case NodeTypeRoot, NodeTypeEndPoint:
		result[node.(*_Node).path()] = 1
	case NodeTypeRecursive:
		result[node.(*_Node).path()] = 1
	}
}

func (r *Route) String() string {
	traverse := map[string]int{}
	r.traverse(r.root, traverse)
	var paths []string
	for path := range traverse {
		paths = append(paths, path)
	}

	sort.Strings(paths)
	rtn := ""
	for _, path := range paths {
		rtn = fmt.Sprintf("%s\n%s", rtn, path)
	}

	rtn = strings.TrimLeft(rtn, "\n")
	return rtn
}

func (r *Route) SetRootHandlers(handlers ...Handler) *Route {
	r.root.(*_Node).handlers = handlers
	return r
}

func (r *Route) SetEndpoint(path string, handlers ...Handler) *Route {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	if path == "" {
		r.root.(*_Node).handlers = handlers
		return r
	}

	current := r.root
	parts := strings.Split(path, "/")
	partsLen := len(parts)
	for idx, part := range parts {
		if strings.Index(part, ":") == 0 {
			current.(*_Node).nodeType = NodeTypeEndPoint
			current.(*_Node).parameterName = part[1:]
			if idx+1 == partsLen {
				current.(*_Node).handlers = handlers
			}

			continue
		}

		if part == "*" {
			current.(*_Node).nodeType = NodeTypeRecursive
			current.(*_Node).parameterName = current.Name()
			current.(*_Node).handlers = handlers
			return r
		}

		if v, f := current.Children()[part]; f {
			current = v
		} else {
			node := &_Node{
				parent:        current,
				name:          part,
				parameterName: "",
				handlers:      []Handler{},
				children:      map[string]Node{},
				nodeType:      NodeTypeNamespace,
			}

			if idx+1 == partsLen {
				node.nodeType = NodeTypeEndPoint
				node.parameterName = part
				node.handlers = handlers
			}

			current.Children()[part] = node
			current = node
		}
	}

	return r
}

func (r *Route) FindNode(path string) Node {
	routeNode, _, _ := r.RouteNode(path)
	return routeNode
}

func (r *Route) RouteNode(path string) (node Node, parameters map[string]string, isLast bool) {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	params := map[string]string{}
	if path == "" {
		return r.root, nil, true
	}

	parts := strings.Split(path, "/")
	nodeLens := len(parts)
	current := r.root
	next := r.root
	for idx, part := range parts {
		next = current.Children()[part]
		switch current.NodeType() {
		case NodeTypeRoot, NodeTypeEndPoint:
			if idx+1 == nodeLens {
				if next == nil {
					if current == r.root && part != "" {
						return nil, nil, false
					} else {
						params[current.ParameterName()] = part
						return current, params, false
					}
				} else {
					return next, params, true
				}
			} else {
				if next == nil {
					if _, f := current.Children()[parts[idx+1]]; f {
						params[current.ParameterName()] = part
						continue
					} else {
						return nil, nil, false
					}
				} else {
					current = next
				}
			}
		case NodeTypeRecursive:
			if next == nil {
				params[current.ParameterName()] = part
			}

			return current, params, false
		case NodeTypeNamespace:
			if next == nil {
				return nil, nil, false
			}

			current = next
		}
	}

	return current, params, current == next
}
