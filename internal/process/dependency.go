package process

import (
	"fmt"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// TopologicalSort returns services in dependency order
func TopologicalSort(services map[string]config.Service) ([]string, error) {
	// Build adjacency list
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for name := range services {
		inDegree[name] = 0
	}

	for name, svc := range services {
		for _, dep := range svc.DependsOn {
			if _, exists := services[dep]; !exists {
				return nil, fmt.Errorf("unknown dependency: %s depends on %s", name, dep)
			}
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Kahn's algorithm
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var result []string
	for len(queue) > 0 {
		// Pop from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Reduce in-degree of dependents
		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Check for cycles
	if len(result) != len(services) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// ReverseDependencyOrder returns services in reverse dependency order (for shutdown)
func ReverseDependencyOrder(order []string) []string {
	n := len(order)
	reversed := make([]string, n)
	for i, name := range order {
		reversed[n-1-i] = name
	}
	return reversed
}

// GetDependencies returns all transitive dependencies of a service
func GetDependencies(services map[string]config.Service, name string) []string {
	visited := make(map[string]bool)
	var result []string

	var visit func(n string)
	visit = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true

		svc, ok := services[n]
		if !ok {
			return
		}

		for _, dep := range svc.DependsOn {
			visit(dep)
		}

		result = append(result, n)
	}

	visit(name)

	// Remove the service itself from dependencies
	if len(result) > 0 {
		result = result[:len(result)-1]
	}

	return result
}

// GetDependents returns all services that depend on the given service
func GetDependents(services map[string]config.Service, name string) []string {
	var result []string

	for svcName, svc := range services {
		for _, dep := range svc.DependsOn {
			if dep == name {
				result = append(result, svcName)
				break
			}
		}
	}

	return result
}
