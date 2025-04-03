package minmaxheap

import (
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	seed       int64
	globalRand *rand.Rand
	randMu     sync.Mutex
)

func init() {
	flag.Int64Var(&seed, "seed", 0, "Random seed (default is current time)")
}

func TestMain(m *testing.M) {
	flag.Parse()

	randMu.Lock()
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	globalRand = rand.New(rand.NewSource(seed)) // seeded once for all test-local RNGs
	randMu.Unlock()

	os.Exit(m.Run())
}

// newTestRand creates a deterministic *rand.Rand for the given test based on the test name.
func newTestRand(t *testing.T) *rand.Rand {
	randMu.Lock()
	defer randMu.Unlock()

	t.Logf("using global seed %d", seed)
	h := sha256.Sum256([]byte(t.Name()))
	namePart := int64(binary.BigEndian.Uint64(h[:8]))
	nameSeed := seed ^ namePart // xor to combine them
	return rand.New(rand.NewSource(nameSeed))
}

type myHeap []int

func (h myHeap) Len() int           { return len(h) }
func (h myHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h myHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *myHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (h *myHeap) Push(x interface{}) {
	*h = append(*h, x.(int))
}

func (h myHeap) verify(t *testing.T, i int) {
	t.Helper()
	n := h.Len()
	l := 2*i + 1
	r := 2*i + 2
	childrenAndGrandchildren := []int{
		l,       // left child
		r,       // right child
		2*l + 1, // left child of left child
		2*l + 2, // right child of left child
		2*r + 1, // left child of right child
		2*r + 2, // right child of right child
	}

	for cNum, descendant := range childrenAndGrandchildren {
		if descendant >= n {
			continue
		}
		if isMinLevel(i) {
			if h.Less(descendant, i) {
				filename := h.Format(t, i)
				t.Fatalf("heap invariant violated [%d] = %d >= [%d] = %d\n  SVG rendering of tree can be found at %s", i, h[i], descendant, h[descendant], filename)
			}
		} else {
			if h.Less(i, descendant) {
				filename := h.Format(t, descendant)
				t.Fatalf("heap invariant violated [%d] = %d >= [%d] = %d\n  SVG rendering of tree can be found at %s", descendant, h[descendant], i, h[i], filename)
			}
		}
		if cNum < 2 {
			// only recurse to immediate children
			h.verify(t, descendant)
		}
	}
}

func (h myHeap) Format(t *testing.T, highlight int) (filename string) {
	if h.Len() == 0 {
		return "<no image; empty heap>"
	}

	// Generate SVG representation of the heap
	svgContent := h.FormatSVG(highlight)

	// Create the output directory if it doesn't exist
	outputDir := "heap_visualizations"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Errorf("Error creating output directory: %v", err)
		return
	}

	// Write SVG to a file in the output directory
	timestamp := time.Now().Format("20060102_150405.000000")
	filename = filepath.Join(outputDir, fmt.Sprintf("%s_nodes%d_highlight%d_%s.svg",
		t.Name(), h.Len(), highlight, timestamp))

	if err := os.WriteFile(filename, []byte(svgContent), 0644); err != nil {
		t.Errorf("Error writing SVG file: %v", err)
		return
	}

	return filename
}

// FormatSVG generates an SVG representation of the heap tree
func (h myHeap) FormatSVG(highlight int) string {
	// Determine SVG parameters based on heap size
	var nodeDiameter, levelHeight, leftMargin, topMargin int

	// Adjust parameters based on heap size
	if h.Len() <= 31 { // Small heap (5 levels or fewer)
		nodeDiameter = 40
		levelHeight = 80
		leftMargin = 10
		topMargin = 20
	} else if h.Len() <= 127 { // Medium heap (6-7 levels)
		nodeDiameter = 30
		levelHeight = 60
		leftMargin = 10
		topMargin = 20
	} else { // Large heap (8+ levels)
		nodeDiameter = 24
		levelHeight = 50
		leftMargin = 10
		topMargin = 20
	}

	// Calculate the total width needed for the tree
	levels := level(h.Len())

	// For very large heaps, limit the width by not showing all levels
	maxLevelsToShow := levels
	if levels > 7 {
		maxLevelsToShow = 7 // Only show top 7 levels for very large heaps
	}

	maxNodesInLevel := 1 << maxLevelsToShow // Maximum nodes in the last level we'll show
	totalWidth := maxNodesInLevel*(nodeDiameter*2) + leftMargin*2
	totalHeight := (maxLevelsToShow+1)*levelHeight + topMargin*2

	// Start the SVG
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("<svg width=\"%d\" height=\"%d\" xmlns=\"http://www.w3.org/2000/svg\">\n",
		totalWidth, totalHeight))

	// Add a title
	buf.WriteString(fmt.Sprintf("  <title>MinMaxHeap Visualization (%d nodes)</title>\n", h.Len()))

	// Style definitions
	buf.WriteString(`  <style>
    .node-min { fill: lightblue; stroke: #333; stroke-width: 2; }
    .node-max { fill: lightpink; stroke: #333; stroke-width: 2; }
    .node-highlight { stroke: red; stroke-width: 3; }
    .node-text { font-family: Arial; font-size: 14px; text-anchor: middle; dominant-baseline: middle; }
    .node-index { font-family: Arial; font-size: 10px; text-anchor: middle; fill: #666; }
    .edge { stroke: #666; stroke-width: 1.5; fill: none; }
    .legend { font-family: Arial; font-size: 12px; }
  </style>
`)

	// Add a legend
	buf.WriteString(`  <!-- Legend -->
  <rect x="10" y="10" width="15" height="15" class="node-min" />
  <text x="30" y="22" class="legend">Min Level</text>
  <rect x="100" y="10" width="15" height="15" class="node-max" />
  <text x="120" y="22" class="legend">Max Level</text>
`)

	// Calculate positions for each node
	nodesCount := h.Len()
	if maxLevelsToShow < levels {
		// Calculate how many nodes we're showing if we limited levels
		nodesCount = (1 << (maxLevelsToShow + 1)) - 1
		if nodesCount > h.Len() {
			nodesCount = h.Len()
		}
	}

	nodePositions := make([]struct{ x, y int }, nodesCount)
	for i := 0; i < nodesCount; i++ {
		lev := level(i)
		if lev > maxLevelsToShow {
			continue // Skip nodes beyond our display level
		}

		nodesInLevel := 1 << lev

		// Calculate width of this level
		levelWidth := totalWidth - (leftMargin * 2)

		// Calculate position in this level (0-based)
		posInLevel := i - ((1 << lev) - 1)

		// Calculate x position (centered in its segment)
		segmentWidth := levelWidth / nodesInLevel
		x := leftMargin + (posInLevel * segmentWidth) + (segmentWidth / 2)

		// Calculate y position
		y := topMargin + (lev * levelHeight) + (nodeDiameter / 2)

		nodePositions[i] = struct{ x, y int }{x, y}
	}

	// Draw edges first (so they'll be behind nodes)
	buf.WriteString("  <!-- Edges connecting nodes -->\n")
	for i := 0; i < nodesCount; i++ {
		lev := level(i)
		if lev >= maxLevelsToShow {
			continue // Skip drawing edges from nodes in the last level
		}

		leftChild := 2*i + 1
		rightChild := 2*i + 2

		if leftChild < nodesCount {
			buf.WriteString(fmt.Sprintf("  <path class=\"edge\" d=\"M%d,%d C%d,%d %d,%d %d,%d\" />\n",
				nodePositions[i].x, nodePositions[i].y+(nodeDiameter/2),
				nodePositions[i].x, nodePositions[i].y+levelHeight/3,
				nodePositions[leftChild].x, nodePositions[leftChild].y-levelHeight/3,
				nodePositions[leftChild].x, nodePositions[leftChild].y-(nodeDiameter/2)))
		}

		if rightChild < nodesCount {
			buf.WriteString(fmt.Sprintf("  <path class=\"edge\" d=\"M%d,%d C%d,%d %d,%d %d,%d\" />\n",
				nodePositions[i].x, nodePositions[i].y+(nodeDiameter/2),
				nodePositions[i].x, nodePositions[i].y+levelHeight/3,
				nodePositions[rightChild].x, nodePositions[rightChild].y-levelHeight/3,
				nodePositions[rightChild].x, nodePositions[rightChild].y-(nodeDiameter/2)))
		}
	}

	// Draw all nodes
	buf.WriteString("  <!-- Nodes -->\n")
	for i := 0; i < nodesCount; i++ {
		lev := level(i)
		if lev > maxLevelsToShow {
			continue // Skip nodes beyond our display level
		}

		x := nodePositions[i].x
		y := nodePositions[i].y

		// Determine node class based on min/max level
		nodeClass := "node-min"
		if !isMinLevel(i) {
			nodeClass = "node-max"
		}

		// Add highlight class if needed
		if i == highlight {
			nodeClass += " node-highlight"
		}

		// Draw the node
		buf.WriteString(fmt.Sprintf("  <circle cx=\"%d\" cy=\"%d\" r=\"%d\" class=\"%s\" />\n",
			x, y, nodeDiameter/2, nodeClass))

		// Add the value text
		buf.WriteString(fmt.Sprintf("  <text x=\"%d\" y=\"%d\" class=\"node-text\">%d</text>\n",
			x, y, h[i]))

		// Add node index for all nodes as a reference
		fontSize := 10
		if nodeDiameter < 30 {
			fontSize = 8 // Smaller font for smaller nodes
		}

		buf.WriteString(fmt.Sprintf("  <text x=\"%d\" y=\"%d\" dy=\"%d\" class=\"node-index\" font-size=\"%d\">[%d]</text>\n",
			x, y, -nodeDiameter/2-2, fontSize, i))
	}

	// If we limited the display, add a note
	if maxLevelsToShow < levels {
		buf.WriteString(fmt.Sprintf("  <text x=\"%d\" y=\"%d\" class=\"legend\">Note: Only showing %d of %d levels. Total nodes: %d</text>\n",
			totalWidth/2, totalHeight-20, maxLevelsToShow, levels, h.Len()))
	}

	// End the SVG
	buf.WriteString("</svg>")

	return buf.String()
}

func TestInit0(t *testing.T) {
	h := new(myHeap)
	for i := 20; i > 0; i-- {
		h.Push(0) // all elements are the same
	}
	Init(h)
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := Pop(h).(int)
		h.verify(t, 0)
		if x != 0 {
			t.Errorf("%d.th pop got %d; want %d", i, x, 0)
		}
	}
}

func TestInit0Max(t *testing.T) {
	h := new(myHeap)
	for i := 20; i > 0; i-- {
		h.Push(0) // all elements are the same
	}
	Init(h)
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := PopMax(h).(int)
		h.verify(t, 0)
		if x != 0 {
			t.Errorf("%d.th popmax got %d; want %d", i, x, 0)
		}
	}
}

func TestInit1(t *testing.T) {
	h := new(myHeap)
	for i := 20; i > 0; i-- {
		h.Push(i) // all elements are different
	}
	Init(h)
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := Pop(h).(int)
		h.verify(t, 0)
		if x != i {
			t.Errorf("%d.th pop got %d; want %d", i, x, i)
		}
	}
}

func TestInit1Max(t *testing.T) {
	h := new(myHeap)
	for i := 20; i > 0; i-- {
		h.Push(i) // all elements are different
	}
	Init(h)
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := PopMax(h).(int)
		h.verify(t, 0)
		if x != 20-i+1 {
			t.Errorf("%d.th pop got %d; want %d", i, x, 20-i+1)
		}
	}
}
func TestInit2(t *testing.T) {
	testcases := []myHeap{
		{6, 10, 13, 3, 12, 8, 12, 2, 12, 16},
	}
	for _, tc := range testcases {
		Init(&tc)
		tc.verify(t, 0)
	}
}

func Test(t *testing.T) {
	h := new(myHeap)
	h.verify(t, 0)

	for i := 20; i > 10; i-- {
		h.Push(i)
	}
	Init(h)
	h.verify(t, 0)

	for i := 10; i > 0; i-- {
		Push(h, i)
		h.verify(t, 0)
	}

	for i := 1; h.Len() > 0; i++ {
		x := Pop(h).(int)
		if i < 20 {
			Push(h, 20+i)
		}
		h.verify(t, 0)
		if x != i {
			t.Errorf("%d.th pop got %d; want %d", i, x, i)
		}
	}
}

func TestMax(t *testing.T) {
	h := new(myHeap)
	h.verify(t, 0)

	for i := 20; i > 10; i-- {
		h.Push(i)
	}
	Init(h)
	h.verify(t, 0)

	for i := 10; i > 0; i-- {
		Push(h, i)
		h.verify(t, 0)
	}

	for i := 1; h.Len() > 0; i++ {
		x := PopMax(h).(int)
		if i > 20 {
			Push(h, i-20)
		}
		h.verify(t, 0)
		if x != 20-i+1 {
			t.Errorf("%d.th pop got %d; want %d", i, x, 20-i+1)
		}
	}
}

func TestRandomSorted(t *testing.T) {
	rng := newTestRand(t)

	const n = 1_000
	h := new(myHeap)
	for i := 0; i < n; i++ {
		*h = append(*h, rng.Intn(n/2))
	}

	Init(h)
	h.verify(t, 0)

	var ints []int
	for h.Len() > 0 {
		ints = append(ints, Pop(h).(int))
		h.verify(t, 0)
	}
	if !sort.IntsAreSorted(ints) {
		t.Fatal("min pop order invalid")
	}
}

func TestRandomSortedMax(t *testing.T) {
	rng := newTestRand(t)

	const n = 1_000
	h := new(myHeap)
	for i := 0; i < n; i++ {
		*h = append(*h, rng.Intn(n/2))
	}

	Init(h)
	h.verify(t, 0)

	var ints []int
	for h.Len() > 0 {
		ints = append(ints, PopMax(h).(int))
		h.verify(t, 0)
	}
	if !sort.IsSorted(sort.Reverse(sort.IntSlice(ints))) {
		t.Fatal("max pop order invalid")
	}
}

func TestRemove0(t *testing.T) {
	h := new(myHeap)
	for i := 0; i < 10; i++ {
		Push(h, i)
	}
	h.verify(t, 0)

	for h.Len() > 0 {
		i := h.Len() - 1
		want := (*h)[i]
		x := Remove(h, i).(int)
		if x != want {
			t.Errorf("Remove(%d) got %d; want %d", i, x, want)
		}
		h.verify(t, 0)
	}
}

func TestRemove1(t *testing.T) {
	h := new(myHeap)
	for i := 0; i < 10; i++ {
		Push(h, i)
	}
	h.verify(t, 0)

	for i := 0; h.Len() > 0; i++ {
		x := Remove(h, 0).(int)
		if x != i {
			t.Errorf("Remove(0) got %d; want %d", x, i)
		}
		h.verify(t, 0)
	}
}

func TestRemove2(t *testing.T) {
	N := 10

	h := new(myHeap)
	for i := 0; i < N; i++ {
		Push(h, i)
	}
	h.verify(t, 0)

	m := make(map[int]bool)
	for h.Len() > 0 {
		m[Remove(h, (h.Len()-1)/2).(int)] = true
		h.verify(t, 0)
	}

	if len(m) != N {
		t.Errorf("len(m) = %d; want %d", len(m), N)
	}
	for i := 0; i < len(m); i++ {
		if !m[i] {
			t.Errorf("m[%d] doesn't exist", i)
		}
	}
}

func TestRemove3(t *testing.T) {
	rng := newTestRand(t)
	N := 200

	h := new(myHeap)
	for i := 0; i < N; i++ {
		Push(h, i)
	}
	h.verify(t, 0)

	// remove all in random order
	removed := make(map[int]struct{})
	for h.Len() > 0 {
		i := rng.Intn(h.Len())
		x := Remove(h, i).(int)
		h.verify(t, 0)
		removed[x] = struct{}{}
	}

	// make sure all were removed
	for i := 0; i < N; i++ {
		if _, ok := removed[i]; !ok {
			t.Errorf("value %d was never removed", i)
		}
		delete(removed, i)
	}
	for k := range removed {
		t.Errorf("value %d was removed but never added", k)
	}
}

func BenchmarkDup(b *testing.B) {
	const n = 10000
	h := make(myHeap, 0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			Push(&h, 0) // all elements are the same
		}
		for h.Len() > 0 {
			Pop(&h)
		}
	}
}

func TestFix0(t *testing.T) {
	rng := newTestRand(t)

	h := new(myHeap)
	h.verify(t, 0)

	for i := 200; i > 0; i -= 10 {
		Push(h, i)
	}
	h.verify(t, 0)

	if (*h)[0] != 10 {
		t.Fatalf("Expected head to be 10, was %d", (*h)[0])
	}
	(*h)[0] = 210
	Fix(h, 0)
	h.verify(t, 0)

	for i := 100; i > 0; i-- {
		elem := rng.Intn(h.Len())
		if i&1 == 0 {
			(*h)[elem] *= 2
		} else {
			(*h)[elem] /= 2
		}
		Fix(h, elem)
		h.verify(t, 0)
	}
}

func TestFix1(t *testing.T) {
	h := new(myHeap)

	for i := 0; i < 100; i++ {
		Push(h, 100-i)
		h.verify(t, 0)
	}
	(*h)[48] = -1
	Fix(h, 48)
	h.verify(t, 0)
	got := Pop(h).(int)
	if got != -1 {
		t.Fatalf("expected -1 as minimum, got %d", got)
	}
}
