package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Op encode an operation that can be perfomed on the source digits.
type Op byte

const (
	OpNull Op = iota
	OpAdd     // Binary ops start here
	OpSub
	OpMul
	OpDiv
	OpPow
	OpFact // Unary ops start here
	OpSqrt
	OpMinus // unary minus
)

var opNames = map[Op]string{
	OpNull:  "null",
	OpAdd:   "+",
	OpSub:   "-",
	OpMul:   "*",
	OpDiv:   "/",
	OpPow:   "^",
	OpFact:  "!",
	OpSqrt:  "sqrt",
	OpMinus: "--",
}

// unary returns true for unary operators
func (op Op) unary() bool {
	return op >= OpFact
}

// binary returns true for binary operators
func (op Op) binary() bool {
	return op >= OpAdd && op <= OpPow
}

// String returns string representation for op
func (op Op) String() string {
	return opNames[op]
}

// Value defines an interface for anything on which above operations can be performed.
type Value interface {
	PerformUnary(Op) (Value, error)
	PerformBinary(Op, Value) (Value, error)
	Equal(Value) bool
	Less(Value) bool
	String() string
	IsInteger() bool
	Negative() bool
	// The following methods are needed for some simplifications
	Even() bool
	Zero() bool
	One() bool
	MinusOne() bool
}

// Node represents a formula parse tree, storing value (for a leaf) or
// operand with left and right sub-nodes. Nodes with unary operators will have their
// right sub-node nil, which is checked by Node.valid().
type Node struct {
	left, right *Node
	val         Value
	op          Op
}

// valid returns true for correct nodes. It does NOT check the subnodes recursively.
func (n *Node) valid() bool {
	if n.op == OpNull {
		return n.left == nil && n.right == nil
	} else if n.op <= OpPow {
		return n.left != nil && n.right != nil
	} else {
		return n.left != nil && n.right == nil
	}
}

// newNode creates a new formula Node. It panics if requested Node will be not valid.
func newNode(left *Node, op Op, right *Node) *Node {
	n := &Node{left: left, op: op, right: right}
	if !n.valid() {
		panic(fmt.Sprintf("Cannot create non-valid node: %v %v %v", left, op, right))
	}
	return n
}

// newValNode creates a new value Node from a rational.
func newValNode(val Value) *Node {
	return &Node{val: val}
}

// newIntNode creates a new value Node from an integer.
func newIntNode(val int64) *Node {
	r, _ := newRational(val, 1)
	return &Node{val: r}
}

// FromPolish parses a node from a string, and returns an error if the input is invalid.
// The input should be in Polish notation, with operands possibly separated by one or several spaces,
// and rational numbers writen as a/b without any spaces around '/'. To avoid ambiguity,
// unary minus should be encoded as --.
// It reads as much as possible. See tests for some examples.
func FromPolish(s string) (*Node, error) {
	s = strings.TrimSpace(s)
	nd, _, err := parseNodeFromString(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse '%s': %s", s, err)
	}
	return nd, nil
}

// ToPolish is an opposite of FromPolish: it returns a node writen in the polish notation.
func (n *Node) ToPolish() string {
	if !n.valid() {
		return fmt.Sprintf("invalid formula: '%s'", n)
	}
	if n.op == OpNull {
		return n.val.String()
	} else {
		var s string
		if n.op != OpMinus {
			s = n.op.String()
		} else {
			s = "--"
		}
		s += " " + n.left.ToPolish()
		if n.op.binary() {
			s += " " + n.right.ToPolish()
		}
		return s
	}
}

// parseNodeFromString does heavy lifting for parseString. It parses as much as possible and
// returns the node parsed and the remainder of the string.
func parseNodeFromString(s string) (*Node, string, error) {
	s = strings.TrimSpace(s)
	// Try to parse rational first
	if ind := ratRx.FindStringIndex(s); ind != nil {
		v, err := newRationalFromString(strings.TrimSpace(s[:ind[1]]))
		if err != nil {
			return nil, s[ind[1]:], err
		}
		return newValNode(v), s[ind[1]:], nil
	}
	if s == "" {
		return nil, "", fmt.Errorf("empty string")
	}
	var op Op
	if strings.HasPrefix(s, "sqrt") {
		op = OpSqrt
		s = s[4:]
	} else if strings.HasPrefix(s, "--") {
		op = OpMinus
		s = s[2:]
	} else {
		for k := range opNames {
			if opNames[k] == s[:1] {
				op = k
				if op == OpMinus {
					op = OpSub
				}
			}
		}
		if op == OpNull {
			return nil, s[1:], fmt.Errorf("unrecognized operator in '%s'", s)
		}
		s = s[1:]
	}
	if s == "" {
		return nil, s, fmt.Errorf("first operand missing")
	}
	n1, s1, err := parseNodeFromString(s)
	if err != nil {
		return nil, s1, err
	}
	if op.unary() {
		return newNode(n1, op, nil), s1, nil
	} else {
		n2, s2, err := parseNodeFromString(s1)
		if err != nil {
			return nil, s2, fmt.Errorf("second operand missing")
		} else {
			return newNode(n1, op, n2), s2, nil
		}
	}
}

var ratRx *regexp.Regexp // Regular expression for a rational number

func init() {
	ratRx = regexp.MustCompile("^\\s*-?[0-9]+(/[0-9]+)?")
}

// Depth returns distance of the deepest leaf to the root.
func (n *Node) Depth() int64 {
	if n.op == OpNull {
		return 0
	}
	depth := n.left.Depth()
	if n.right != nil {
		if d := n.right.Depth(); d > depth {
			depth = d
		}
	}
	return depth + 1
}

// Equal returns true if two nodes have identical structure and leafs.
func (n *Node) Equal(n1 *Node) bool {
	if n1 == nil || n.op != n1.op {
		return false
	}
	if n.op != OpNull {
		return n.left.Equal(n1.left) && (n.right == nil || n.right.Equal(n1.right))
	} else {
		return n.val.Equal(n1.val)
	}
}

// Eval evaluates formula value, and raises an error if the result is invalid
// or cannot be represented by a rational.
func (n *Node) Eval() (Value, error) {
	if !n.valid() {
		return rational{}, fmt.Errorf("invalid formula %s", n)
	}
	if n.op == OpNull {
		return n.val, nil
	} else if n.op.binary() {
		left, err := n.left.Eval()
		if err != nil {
			return n.val, err
		}
		right, err := n.right.Eval()
		if err != nil {
			return n.val, err
		}
		return left.PerformBinary(n.op, right)
	} else {
		left, err := n.left.Eval()
		if err != nil {
			return n.val, err
		}
		return left.PerformUnary(n.op)
	}
}

// transformDuo transorms all expressions of the form (op1 a) op2 (op3 b) into op4 (a op5 b),
// and leaves other expressions intact. In the form above, (OpNull x) is treated as x.
func (n *Node) transformDuo(op1, op2, op3, op4, op5 Op) *Node {
	var a, b *Node
	if n.op == op2 {
		if n.left.op == op1 && n.left.left != nil {
			a = n.left.left.Simplify()
		} else if op1 == OpNull {
			a = n.left.Simplify()
		} else {
			return n
		}
		if n.right.op == op3 && n.right.left != nil {
			b = n.right.left.Simplify()
		} else if op3 == OpNull {
			b = n.right.Simplify()
		} else {
			return n
		}
		n1 := &Node{op: op5, left: a, right: b}
		if op4 != OpNull {
			n1 = &Node{op: op4, left: n1.Simplify()}
		}
		return n1
	} else {
		return n
	}
}

// transformTrio transforms an expression of the form a op1 (b op2 c) into (a op3 b) op4 c,
// and leaves other expressions intact.
func (n *Node) transformTrio(op1, op2, op3, op4 Op) *Node {
	if n.op == op1 && n.right.op == op2 {
		n1 := &Node{op: op3, left: n.left.Simplify(), right: n.right.left.Simplify()}
		return &Node{op: op4, left: n1.Simplify(), right: n.right.right.Simplify()}
	} else {
		return n
	}
}

// Make various simplifications to convert n into a canonical form.
func (n *Node) Simplify() *Node {
	var n1 *Node
	if n.op == OpMinus && n.left.op == OpMinus {
		n1 = n.left.left.Simplify()
	} else if n.op == OpPow && n.left.op == OpMinus {
		e, err := n.right.Eval()
		if err == nil && e.Even() {
			n1 = &Node{op: OpPow, left: n.left.left.Simplify(), right: n.right.Simplify()}
		} else {
			n1 = n
		}
	} else {
		n1 = n
		for _, t := range [][5]Op{
			{OpNull, OpAdd, OpMinus, OpNull, OpSub},
			{OpNull, OpAdd, OpMinus, OpNull, OpSub},
			{OpNull, OpSub, OpMinus, OpNull, OpAdd},
			{OpMinus, OpSub, OpNull, OpMinus, OpAdd},
			{OpMinus, OpAdd, OpNull, OpMinus, OpSub},
			{OpMinus, OpMul, OpMinus, OpNull, OpMul},
			{OpMinus, OpDiv, OpMinus, OpNull, OpDiv},
			{OpMinus, OpMul, OpNull, OpMinus, OpMul},
			{OpMinus, OpDiv, OpNull, OpMinus, OpDiv},
			{OpNull, OpMul, OpMinus, OpMinus, OpMul},
			{OpNull, OpDiv, OpMinus, OpMinus, OpDiv},
			{OpSqrt, OpMul, OpSqrt, OpSqrt, OpMul},
			{OpSqrt, OpDiv, OpSqrt, OpSqrt, OpDiv},
		} {
			n1 = n1.transformDuo(t[0], t[1], t[2], t[3], t[4])
		}
		for _, t := range [][4]Op{
			{OpAdd, OpAdd, OpAdd, OpAdd},
			{OpSub, OpSub, OpSub, OpAdd},
			{OpMul, OpMul, OpMul, OpMul},

			{OpDiv, OpDiv, OpDiv, OpMul},
		} {
			n1 = n1.transformTrio(t[0], t[1], t[2], t[3])
		}
		if n1 == n {
			var l, r *Node
			if n.left != nil {
				l = n.left.Simplify()
			}
			if n.right != nil {
				r = n.right.Simplify()
			}
			if l != n.left || r != n.right {
				n1 = &Node{op: n.op, val: n.val, left: l, right: r}
			}
		}
	}
	if n1 != nil && n1 != n {
		return n1.Simplify()
	} else {
		return n
	}
}
