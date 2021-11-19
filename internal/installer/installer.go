package installer

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Component struct {
	ID    string
	Needs []string
}
type Components []Component

type Manifest struct {
	Components
}

type Workload func() error

// Run  TODO
func (c Component) Run() error {
	return nil
}

// Delete removes a components from the tree. It is removed from components
// and as a dependency of all other components.
func (components Components) Delete(id string) Components {
	newComponents := Components{}
	for _, component := range components {
		if component.ID != id { // Keepers
			newNeeds := []string{}
			for _, dependency := range component.Needs {
				if dependency != id { // Keepers
					newNeeds = append(newNeeds, dependency)
				}
			}
			component.Needs = newNeeds
			newComponents = append(newComponents, component)
		}
	}
	return newComponents
}

// Validate does a dry run on the Manifest to check if it can be parsed.
// If not, it returns an error. The tree (manifest) can't be parsed when there
// are circular dependencies.
func (m *Manifest) Validate() error {
	componentsToRun := m.Components
	doneChan := make(chan string)
	errChan := make(chan error)
	var wg sync.WaitGroup

	startedRoutines := m.Components.RunWhatPossible(doneChan, errChan, &wg)

	for len(componentsToRun) > 0 && startedRoutines > 0 {
		select {
		case doneComponent := <-doneChan:
			fmt.Printf("Component %s was done\n", doneComponent)
			startedRoutines -= 1
			componentsToRun = componentsToRun.Delete(doneComponent)
			startedRoutines += componentsToRun.RunWhatPossible(doneChan, errChan, &wg)
		case err := <-errChan:
			// TODO: Receive the WaitGroup from the caller? The caller decided wether to wait or not
			// for started routines to finish. We can simply return an error
			wg.Wait()
			return err
		}
	}

	if len(componentsToRun) != 0 {
		leftOverIDs := []string{}
		for _, c := range componentsToRun {
			leftOverIDs = append(leftOverIDs, c.ID)
		}
		return errors.New("can't run all components. Circular dependency? Left over components: " + strings.Join(leftOverIDs, ","))
	}

	return nil
}

// RunWhatPossible spins up a go routine to run any component that doesn't have
// pending dependencies. Returns the number of go routines started.
func (components Components) RunWhatPossible(doneChan chan string, errChan chan error, wg *sync.WaitGroup) int {
	started := 0
	for _, c := range components {
		if len(c.Needs) == 0 {
			started += 1
			wg.Add(1)
			fmt.Println("Will run " + c.ID)
			go func(comp Component) {
				err := comp.Run()
				if err != nil {
					errChan <- err
				}
				doneChan <- comp.ID
			}(c)
		}
	}
	return started
}
