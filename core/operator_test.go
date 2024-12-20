package core

import (
	"testing"

	"github.com/jizhilong/branch-bot/git"
	"github.com/jizhilong/branch-bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeTrainOperator_Add(t *testing.T) {
	testRepo := git.NewTestRepo(t)

	operator := &MergeTrainOperator{
		repo: &testRepo.Repo,
		mergeTrain: &models.MergeTrain{
			ProjectID:  123,
			IssueIID:   456,
			BranchName: "bb-branches/456",
			Members:    make([]models.MergeTrainItem, 0),
		},
	}

	// Get base commit
	baseHash, err := testRepo.RevParse("HEAD")
	require.NoError(t, err)
	base := &models.GitRef{Name: "main", Commit: baseHash}

	t.Run("add first branch", func(t *testing.T) {
		// Create and add first branch
		feature1 := testRepo.CreateBranch(base, "feature1", "file1.txt", "feature1 content")
		result, fail := operator.Add(feature1)
		assert.NotNil(t, result)
		assert.Nil(t, fail)
		assert.Len(t, operator.mergeTrain.Members, 1)
		assert.Equal(t, "feature1", operator.mergeTrain.Members[0].Branch)
	})

	t.Run("add second branch without conflict", func(t *testing.T) {
		// Create and add second branch
		feature2 := testRepo.CreateBranch(base, "feature2", "file2.txt", "feature2 content")
		result, fail := operator.Add(feature2)
		assert.NotNil(t, result)
		assert.Nil(t, fail)
		assert.Len(t, operator.mergeTrain.Members, 2)
		assert.Equal(t, "feature2", operator.mergeTrain.Members[1].Branch)
	})

	t.Run("update existing branch", func(t *testing.T) {
		// Update feature1 with new content
		feature1 := testRepo.UpdateBranch("feature1", "file1.txt", "updated content")
		result, fail := operator.Add(feature1)
		assert.NotNil(t, result)
		assert.Nil(t, fail)
		assert.Len(t, operator.mergeTrain.Members, 2)
		// feature1 should be moved to the end
		assert.Equal(t, "feature1", operator.mergeTrain.Members[1].Branch)
	})

	t.Run("add branch with conflict", func(t *testing.T) {
		// Create a branch that conflicts with feature1
		conflict := testRepo.CreateBranch(base, "conflict", "file1.txt", "conflicting content")
		result, fail := operator.Add(conflict)
		assert.Nil(t, result)
		assert.NotNil(t, fail)
		// MergeTrain should remain unchanged
		assert.Len(t, operator.mergeTrain.Members, 2)
		if mergeFail, ok := fail.(*models.GitMergeFailResult); assert.True(t, ok) {
			assert.NotEmpty(t, mergeFail.FailedFiles)
			assert.Equal(t, "file1.txt", mergeFail.FailedFiles[0].Path)
		}
	})
}

func TestMergeTrainOperator_Remove(t *testing.T) {
	testRepo := git.NewTestRepo(t)

	// Create initial merge train with three branches
	operator := &MergeTrainOperator{
		repo: &testRepo.Repo,
		mergeTrain: &models.MergeTrain{
			ProjectID:  123,
			IssueIID:   456,
			BranchName: "bb-branches/456",
			Members:    make([]models.MergeTrainItem, 0),
		},
	}

	// Get base commit
	baseHash, err := testRepo.RevParse("HEAD")
	require.NoError(t, err)
	base := &models.GitRef{Name: "main", Commit: baseHash}

	// Create and add three branches
	feature1 := testRepo.CreateBranch(base, "feature1", "file1.txt", "feature1 content")
	feature2 := testRepo.CreateBranch(base, "feature2", "file2.txt", "feature2 content")
	feature3 := testRepo.CreateBranch(base, "feature3", "file3.txt", "feature3 content")

	// Add branches to merge train
	result, fail := operator.Add(feature1)
	require.NotNil(t, result)
	require.Nil(t, fail)
	result, fail = operator.Add(feature2)
	require.NotNil(t, result)
	require.Nil(t, fail)
	result, fail = operator.Add(feature3)
	require.NotNil(t, result)
	require.Nil(t, fail)

	t.Run("remove non-existent branch", func(t *testing.T) {
		result, fail := operator.Remove("non-existent")
		assert.Nil(t, result)
		assert.NotNil(t, fail)
		// MergeTrain should remain unchanged
		assert.Len(t, operator.mergeTrain.Members, 3)
	})

	t.Run("remove middle branch", func(t *testing.T) {
		result, fail := operator.Remove("feature2")
		assert.NotNil(t, result)
		assert.Nil(t, fail)
		// Check remaining members
		assert.Len(t, operator.mergeTrain.Members, 2)
		assert.Equal(t, "feature1", operator.mergeTrain.Members[0].Branch)
		assert.Equal(t, "feature3", operator.mergeTrain.Members[1].Branch)
	})

	t.Run("remove first branch", func(t *testing.T) {
		result, fail := operator.Remove("feature1")
		assert.NotNil(t, result)
		assert.Nil(t, fail)
		// Check remaining members
		assert.Len(t, operator.mergeTrain.Members, 1)
		assert.Equal(t, "feature3", operator.mergeTrain.Members[0].Branch)
	})

	t.Run("remove last branch", func(t *testing.T) {
		result, fail := operator.Remove("feature3")
		assert.Nil(t, result)
		assert.Nil(t, fail)
		// Check members are empty
		assert.Empty(t, operator.mergeTrain.Members)
	})

	t.Run("remove from empty train", func(t *testing.T) {
		result, fail := operator.Remove("feature1")
		assert.Nil(t, result)
		assert.NotNil(t, fail)
		// MergeTrain should remain empty
		assert.Empty(t, operator.mergeTrain.Members)
	})
}

func TestLoadMergeTrainOperator(t *testing.T) {
	testRepo := git.NewTestRepo(t)
	repo := &testRepo.Repo
	// Get base commit
	baseHash, err := testRepo.RevParse("HEAD")
	require.NoError(t, err)
	base := &models.GitRef{Name: "main", Commit: baseHash}
	branchName := "bb-branches/456"

	t.Run("load non-existent merge train", func(t *testing.T) {
		operator, err := LoadMergeTrainOperator(repo, branchName, 123, 456)
		require.NoError(t, err)
		assert.NotNil(t, operator)
		assert.Equal(t, 123, operator.mergeTrain.ProjectID)
		assert.Equal(t, 456, operator.mergeTrain.IssueIID)
		assert.Equal(t, branchName, operator.mergeTrain.BranchName)
		assert.Empty(t, operator.mergeTrain.Members)
	})

	t.Run("load existing merge train", func(t *testing.T) {
		// Create a merge train with some members
		operator := &MergeTrainOperator{
			repo: repo,
			mergeTrain: &models.MergeTrain{
				ProjectID:  123,
				IssueIID:   456,
				BranchName: branchName,
				Members:    make([]models.MergeTrainItem, 0),
			},
		}
		addAndCheckLoaded := func(ref *models.GitRef) {
			result, fail := operator.Add(ref)
			require.NotNil(t, result)
			require.Nil(t, fail)
			// Load the merge train
			loadedOperator, err := LoadMergeTrainOperator(repo, branchName, 123, 456)
			require.NoError(t, err)
			assert.NotNil(t, loadedOperator)
			assert.Equal(t, operator.mergeTrain.ProjectID, loadedOperator.mergeTrain.ProjectID)
			assert.Equal(t, operator.mergeTrain.IssueIID, loadedOperator.mergeTrain.IssueIID)
			assert.Equal(t, operator.mergeTrain.BranchName, loadedOperator.mergeTrain.BranchName)
			assert.Equal(t, len(operator.mergeTrain.Members), len(loadedOperator.mergeTrain.Members))
			for i, member := range operator.mergeTrain.Members {
				assert.Equal(t, member.Branch, loadedOperator.mergeTrain.Members[i].Branch)
				assert.Equal(t, member.MergedCommit, loadedOperator.mergeTrain.Members[i].MergedCommit)
			}
		}

		// Add some branches
		feature1 := testRepo.CreateBranch(base, "feature1", "file1.txt", "feature1 content")
		feature2 := testRepo.CreateBranch(base, "feature2", "file2.txt", "feature2 content")
		addAndCheckLoaded(base)
		addAndCheckLoaded(feature1)
		addAndCheckLoaded(feature2)
	})
}
