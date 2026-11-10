// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newTaggedRepo creates a local repository with two commits, where the first one
// carries an annotated tag v1.0.0 and HEAD points to the second one.
func newTaggedRepo(t *testing.T) (*git.Repository, plumbing.Hash) {
	t.Helper()

	repo, err := git.PlainInit(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	signature := &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}

	first, err := worktree.Commit("first", &git.CommitOptions{AllowEmptyCommits: true, Author: signature})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.CreateTag("v1.0.0", first, &git.CreateTagOptions{
		Message: "release v1.0.0",
		Tagger:  signature,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Commit("second", &git.CommitOptions{AllowEmptyCommits: true, Author: signature})
	if err != nil {
		t.Fatal(err)
	}

	return repo, first
}

func TestFindReferenceForAnnotatedTag(t *testing.T) {
	repo, tagged := newTaggedRepo(t)

	hash, err := repo.ResolveRevision(plumbing.Revision("v1.0.0"))
	if err != nil {
		t.Fatalf("resolving the annotated tag: %v", err)
	}
	if *hash != tagged {
		t.Fatalf("annotated tag resolved to %s, expected its commit %s", hash, tagged)
	}

	ref, err := findReferenceForRevision(repo, "v1.0.0", *hash)
	if err != nil {
		t.Fatalf("finding the reference for the tag: %v", err)
	}
	if ref.Name().Short() != "v1.0.0" {
		t.Errorf("reference name is %q, expected the tag name v1.0.0", ref.Name().Short())
	}
	if ref.Hash() != tagged {
		t.Errorf("reference hash is %s, expected the tagged commit %s", ref.Hash(), tagged)
	}
}

func TestFindReferenceForCommitSHA(t *testing.T) {
	repo, tagged := newTaggedRepo(t)

	// the raw commit is not the name of any reference, but it must still be found
	// through the annotated tag that points to it, without failing on the tag object
	ref, err := findReferenceForRevision(repo, tagged.String(), tagged)
	if err != nil {
		t.Fatalf("finding the reference for a commit sha: %v", err)
	}
	if ref.Hash() != tagged {
		t.Errorf("reference hash is %s, expected the commit %s", ref.Hash(), tagged)
	}
}
