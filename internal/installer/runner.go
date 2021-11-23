package installer

import (
	"context"
	"fmt"
	"os"
	"sync"
)

type Action interface {
	Apply(context.Context, Component) error
}

// Walk all the nodes, apply Action and wait for it to finish. Walk nodes in parallel, if parents ("needs") are done.
func Walk(ctx context.Context, plan Components, action Action) {
	done := map[DeploymentID]bool{}
	running := map[DeploymentID]bool{}
	for _, c := range plan {
		done[c.ID] = false
		running[c.ID] = false
	}

	wg := &sync.WaitGroup{}
	var lock = &sync.RWMutex{}
	for !allDone(lock, done) {
		for _, c := range plan {
			c := c
			lock.RLock()
			if done[c.ID] {
				//fmt.Printf("skip done: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if running[c.ID] {
				//fmt.Printf("skip running: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if c.Needs != "" && !done[c.Needs] {
				//fmt.Printf("skip '%s' for deps: %s (r:%v, d:%v)\n", c.ID, c.Needs, running[c.Needs], done[c.Needs])
				lock.RUnlock()
				continue
			}
			lock.RUnlock()

			//fmt.Printf("did not skip: %s\n", c.ID)
			lock.Lock()
			running[c.ID] = true
			lock.Unlock()

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()

				if err := action.Apply(ctx, c); err != nil {
					fmt.Println(err)
					os.Exit(-1)
				}

				lock.Lock()
				done[c.ID] = true
				lock.Unlock()
			}(wg)
		}
	}

	wg.Wait()
}

// ReverseWalk all the nodes, apply Action and wait for it to finish. Walk nodes in parallel, blocks if node still has running needers
func ReverseWalk(ctx context.Context, plan Components, action Action) {
	done := map[DeploymentID]bool{}
	running := map[DeploymentID]bool{}
	for _, c := range plan {
		done[c.ID] = false
		running[c.ID] = false
	}

	needers := map[DeploymentID][]DeploymentID{}
	for _, c := range plan {
		if c.Needs != "" {
			needers[c.Needs] = append(needers[c.Needs], c.ID)
		}
	}

	wg := &sync.WaitGroup{}
	var lock = &sync.RWMutex{}
	for !allDone(lock, done) {
		for _, c := range plan {
			c := c
			lock.RLock()
			if done[c.ID] {
				lock.RUnlock()
				continue
			}
			if running[c.ID] {
				lock.RUnlock()
				continue
			}
			if len(needers[c.ID]) > 0 {
				blocked := false
				for _, n := range needers[c.ID] {
					if !done[n] {
						blocked = true
						break
					}

				}
				if blocked {
					lock.RUnlock()
					continue
				}
			}
			lock.RUnlock()

			lock.Lock()
			running[c.ID] = true
			lock.Unlock()

			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				defer wg.Done()

				if err := action.Apply(ctx, c); err != nil {
					fmt.Println(err)
					os.Exit(-1)
				}

				lock.Lock()
				done[c.ID] = true
				lock.Unlock()
			}(wg)
		}
	}

	wg.Wait()
}

func allDone(lock *sync.RWMutex, s map[DeploymentID]bool) bool {
	lock.RLock()
	defer lock.RUnlock()
	for _, done := range s {
		if !done {
			return false
		}
	}
	return true
}
