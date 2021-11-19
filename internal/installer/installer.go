package installer

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type WorkloadFunc func() error

type Component struct {
	ID       string
	Needs    []string
	Running  bool
	Workload WorkloadFunc
}
type Components []*Component

type Manifest struct {
	Components
}

func (c *Component) Run() error {
	if c.Workload != nil {
		return c.Workload()
	} else {
		fmt.Println("dry run " + c.ID)
	}
	return nil
}

// Delete removes a components from the tree. It is removed from components
// and as a dependency of all other components.
func (components *Components) Delete(id string) {
	newComponents := *components
	compLen := len(newComponents)
	for i, component := range newComponents {
		if component.ID == id { // the component to delete (https://github.com/golang/go/wiki/SliceTricks#delete)
			newComponents = append(newComponents[:i], newComponents[i+1:compLen]...)
			newComponents = newComponents[:compLen-1]
			*components = newComponents // assigning the new slice to the pointed value before returning
		}
	}
	components = &newComponents

	// Now remove it from dependencies
	for _, component := range *components {
		newNeeds := []string{}
		for _, dependency := range component.Needs {
			if dependency != id { // Keepers
				newNeeds = append(newNeeds, dependency)
			}
		}
		component.Needs = newNeeds
	}
}

func (m *Manifest) Validate() error {
	dryRunManifest := *m
	for _, c := range dryRunManifest.Components {
		c.Workload = nil
	}

	return dryRunManifest.Install()
}

// Install runs the Manifest respecting dependencies.
// If it cannot be fully parsed, it returns an error. The tree (manifest) can't be parsed when there
// are circular dependencies.
func (m *Manifest) Install() error {
	componentsToRun := m.Components
	doneChan := make(chan string)
	errChan := make(chan error)
	var wg sync.WaitGroup

	componentsToRun.RunWhatPossible(doneChan, errChan, &wg)

	for len(componentsToRun) > 0 && componentsToRun.StillRunning() != 0 {
		select {
		case doneComponent := <-doneChan:
			fmt.Printf("Component %s was done\n", doneComponent)
			componentsToRun.Delete(doneComponent)
			componentsToRun.RunWhatPossible(doneChan, errChan, &wg)
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

// StillRunning returns the number of components with "Running" == true
func (components *Components) StillRunning() int {
	count := 0
	for _, c := range *components {
		if c.Running {
			count += 1
		}
	}

	return count
}

// RunWhatPossible spins up a go routine to run any component that doesn't have
// pending dependencies.
func (components *Components) RunWhatPossible(doneChan chan string, errChan chan error, wg *sync.WaitGroup) {
	for _, c := range *components {
		if len(c.Needs) == 0 && !c.Running {
			wg.Add(1)
			fmt.Println("Will run " + c.ID)
			c.Running = true
			go func(comp *Component) {
				err := comp.Run()
				if err != nil {
					errChan <- err
				}
				doneChan <- comp.ID
			}(c)
		}
	}
}
