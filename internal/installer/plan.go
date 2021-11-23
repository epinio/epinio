package installer

import "fmt"

// BuildPlan finds a path through the dag, traversing all nodes using Kahn's algorithm
func BuildPlan(components Components) (Components, error) {
	// L ← Empty list that will contain the sorted elements
	plan := make(Components, 0, len(components))

	// S ← Set of all nodes with no incoming edge
	noedge := map[DeploymentID]bool{}

	// graph has all the edges
	graph := map[DeploymentID]DeploymentID{}
	for _, c := range components {
		if c.Needs == "" {
			continue
		}

		graph[c.ID] = c.Needs
	}

	for _, c := range components {
		if graph[c.ID] == "" {
			noedge[c.ID] = true
		}
	}

	// while S is not empty do
	for len(noedge) > 0 {
		//     remove a node n from S
		var n Component
		for _, c := range components {
			if noedge[c.ID] {
				n = c
				delete(noedge, c.ID)
				break
			}
		}

		//     add n to L
		plan = append(plan, n)

		//     for each node m with an edge e from n to m do
		for m, t := range graph {
			//         remove edge e from the graph
			//         if m has no other incoming edges then
			//             insert m into S
			if t == n.ID {
				delete(graph, m)
				// TODO other edges if needs is an array
				noedge[m] = true
			}
		}
	}

	// if graph has edges then
	//     return error   (graph has at least one cycle)
	// else
	//     return L   (a topologically sorted order)

	if len(graph) > 0 {
		return plan, fmt.Errorf("cycle: has edges %v", graph)
	}

	return plan, nil
}
