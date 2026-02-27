package agent

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Fatalf("expected 0, got %d", r.Count())
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	a := New("a1", "Alice")
	if err := r.Register(a); err != nil {
		t.Fatal(err)
	}

	got := r.Get("a1")
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", got.Name)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	a := New("a1", "Alice")
	_ = r.Register(a)
	if err := r.Register(a); err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	if got := r.Get("missing"); got != nil {
		t.Fatal("expected nil for missing agent")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	a := New("a1", "Alice")
	_ = r.Register(a)

	if err := r.Remove("a1"); err != nil {
		t.Fatal(err)
	}
	if r.Count() != 0 {
		t.Fatal("expected 0 after remove")
	}
}

func TestRegistry_RemoveMissing(t *testing.T) {
	r := NewRegistry()
	if err := r.Remove("missing"); err == nil {
		t.Fatal("expected error on remove missing")
	}
}

func TestRegistry_Children(t *testing.T) {
	r := NewRegistry()
	parent := New("p", "Parent")
	_ = r.Register(parent)

	c1 := New("c1", "Child1")
	c1.ParentAgentID = "p"
	c1.Level = 1
	_ = r.Register(c1)

	c2 := New("c2", "Child2")
	c2.ParentAgentID = "p"
	c2.Level = 1
	_ = r.Register(c2)

	other := New("o1", "Other")
	_ = r.Register(other)

	children := r.Children("p")
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}

func TestRegistry_Parent(t *testing.T) {
	r := NewRegistry()
	parent := New("p", "Parent")
	_ = r.Register(parent)

	child := New("c", "Child")
	child.ParentAgentID = "p"
	_ = r.Register(child)

	p := r.Parent("c")
	if p == nil || p.AgentID != "p" {
		t.Fatal("expected parent p")
	}

	// Root has no parent.
	if r.Parent("p") != nil {
		t.Fatal("root should have no parent")
	}
}

func TestRegistry_Lineage(t *testing.T) {
	r := NewRegistry()
	root := New("root", "Root")
	_ = r.Register(root)

	mid := New("mid", "Mid")
	mid.ParentAgentID = "root"
	mid.Level = 1
	_ = r.Register(mid)

	leaf := New("leaf", "Leaf")
	leaf.ParentAgentID = "mid"
	leaf.Level = 2
	_ = r.Register(leaf)

	chain := r.Lineage("leaf")
	if len(chain) != 3 {
		t.Fatalf("expected lineage of 3, got %d", len(chain))
	}
	if chain[0].AgentID != "leaf" || chain[1].AgentID != "mid" || chain[2].AgentID != "root" {
		t.Fatal("unexpected lineage order")
	}
}

func TestRegistry_LineageMissing(t *testing.T) {
	r := NewRegistry()
	if chain := r.Lineage("missing"); chain != nil {
		t.Fatal("expected nil for missing agent lineage")
	}
}

func TestRegistry_Roots(t *testing.T) {
	r := NewRegistry()
	r1 := New("r1", "Root1")
	r2 := New("r2", "Root2")
	c1 := New("c1", "Child1")
	c1.ParentAgentID = "r1"

	_ = r.Register(r1)
	_ = r.Register(r2)
	_ = r.Register(c1)

	roots := r.Roots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestRegistry_Descendants(t *testing.T) {
	r := NewRegistry()
	root := New("root", "Root")
	_ = r.Register(root)

	c1 := New("c1", "Child1")
	c1.ParentAgentID = "root"
	_ = r.Register(c1)

	c2 := New("c2", "Child2")
	c2.ParentAgentID = "root"
	_ = r.Register(c2)

	gc1 := New("gc1", "GrandChild1")
	gc1.ParentAgentID = "c1"
	_ = r.Register(gc1)

	desc := r.Descendants("root")
	if len(desc) != 3 {
		t.Fatalf("expected 3 descendants, got %d", len(desc))
	}
}

func TestRegistry_FindBySpecialization(t *testing.T) {
	r := NewRegistry()
	a := New("a1", "Coder")
	a.Specialization = "code"
	_ = r.Register(a)

	b := New("b1", "Writer")
	b.Specialization = "writing"
	_ = r.Register(b)

	found := r.FindBySpecialization("code")
	if len(found) != 1 || found[0].AgentID != "a1" {
		t.Fatal("expected to find coder agent")
	}
}

func TestRegistry_FindByRole(t *testing.T) {
	r := NewRegistry()
	parent := New("p", "Parent")
	parent.Subagents = []SubagentRef{
		{AgentID: "c1", Role: "coder"},
		{AgentID: "c2", Role: "reviewer"},
	}
	_ = r.Register(parent)

	c1 := New("c1", "Coder")
	c1.ParentAgentID = "p"
	_ = r.Register(c1)

	c2 := New("c2", "Reviewer")
	c2.ParentAgentID = "p"
	_ = r.Register(c2)

	coders := r.FindByRole("coder")
	if len(coders) != 1 || coders[0].AgentID != "c1" {
		t.Fatal("expected to find c1 as coder")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(New("a1", "A"))
	_ = r.Register(New("a2", "B"))
	_ = r.Register(New("a3", "C"))

	if got := len(r.All()); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestRegistry_Depth(t *testing.T) {
	r := NewRegistry()
	if r.Depth() != 0 {
		t.Fatal("expected depth 0 for empty registry")
	}

	root := New("root", "Root")
	root.Level = 0
	_ = r.Register(root)

	child := New("child", "Child")
	child.Level = 1
	child.ParentAgentID = "root"
	_ = r.Register(child)

	gc := New("gc", "GC")
	gc.Level = 2
	gc.ParentAgentID = "child"
	_ = r.Register(gc)

	if d := r.Depth(); d != 3 {
		t.Fatalf("expected depth 3, got %d", d)
	}
}
