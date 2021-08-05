package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type Github interface {
	GetBranches(prefix string, num int) ([]string, error)
	DeleteBranch(refId githubv4.ID) (string, error)
	AllowDeleteProtectedBranch(branchRuleID githubv4.ID, allow bool) (string, error)
	GetBranchProtectionRuleID(prefix string) (githubv4.ID, error)
}

type GithubClient struct {
	Repository string
	Owner      string
	Client     *githubv4.Client
}

type Branch struct {
	ID   githubv4.ID
	Name string
}

func InitClient() (*GithubClient, error) {
	owner := os.Getenv("GITHUB_OWNER")
	repo := os.Getenv("GITHUB_REPO")
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	return &GithubClient{
		Owner:      owner,
		Repository: repo,
		Client:     githubv4.NewClient(httpClient),
	}, nil
}

func (github *GithubClient) GetBranches(prefix string, num int) ([]Branch, error) {
	var query struct {
		Organization struct {
			Repository struct {
				Refs struct {
					Edges []struct {
						Node struct {
							Name githubv4.String
							Id   githubv4.ID
						}
					}
				} `graphql:"refs(refPrefix: $prefix, first: $num)"`
			} `graphql:"repository(name: $repo)"`
		} `graphql:"organization(login: $owner)"`
	}

	vars := map[string]interface{}{
		"owner":  githubv4.String(github.Owner),
		"repo":   githubv4.String(github.Repository),
		"prefix": githubv4.String("refs/heads/" + prefix),
		"num":    githubv4.Int(num),
	}

	err := github.Client.Query(context.Background(), &query, vars)

	if err != nil {
		return nil, fmt.Errorf("failed to query branches: %s", err)
	}

	res := make([]Branch, 0)

	for _, br := range query.Organization.Repository.Refs.Edges {
		res = append(res, Branch{Name: string(br.Node.Name), ID: br.Node.Id})
	}

	return res, nil
}

func (github *GithubClient) GetBranchProtectionRuleID(prefix string) (githubv4.ID, error) {
	var query struct {
		Organization struct {
			Repository struct {
				BranchProtectionRules struct {
					Edges []struct {
						Node struct {
							Id      githubv4.ID
							Pattern githubv4.String
						}
					}
				} `graphql:"branchProtectionRules(first: 10)"`
			} `graphql:"repository(name: $repo)"`
		} `graphql:"organization(login: $owner)"`
	}
	vars := map[string]interface{}{
		"owner": githubv4.String(github.Owner),
		"repo":  githubv4.String(github.Repository),
	}

	err := github.Client.Query(context.Background(), &query, vars)

	if err != nil {
		return "", fmt.Errorf("failed to query branch rules: %s", err)
	}

	for _, br := range query.Organization.Repository.BranchProtectionRules.Edges {
		if strings.HasPrefix(string(br.Node.Pattern), prefix) {
			return br.Node.Id, nil
		}
	}

	return "", fmt.Errorf("could not find branch rule with prefix %s, check your Github settings", prefix)
}

func (github *GithubClient) AllowDeleteProtectedBranch(branchRuleID githubv4.ID, allow githubv4.Boolean) (string, error) {
	var mutation struct {
		UpdateBranchProtectionRule struct {
			BranchProtectionRule struct {
				AllowsDeletions githubv4.Boolean
			}
		} `graphql:"updateBranchProtectionRule(input: $input)"`
	}
	input := githubv4.UpdateBranchProtectionRuleInput{
		BranchProtectionRuleID: branchRuleID,
		AllowsDeletions:        &allow,
	}

	err := github.Client.Mutate(context.Background(), &mutation, input, nil)

	if err != nil {
		return "", fmt.Errorf("failed to mutate branch protection rule: %s", err)
	}

	return fmt.Sprintf("protection rule updated, branch protection is now: %t", allow), nil
}

func (github *GithubClient) DeleteBranch(refId githubv4.ID) (string, error) {
	var mutation struct {
		DeleteRef struct {
			ClientMutationId githubv4.ID
		} `graphql:"deleteRefInput(input: $input)"`
	}

	input := githubv4.DeleteRefInput{
		RefID: refId,
	}

	err := github.Client.Mutate(context.Background(), &mutation, input, nil)

	if err != nil {
		return "", fmt.Errorf("failed to mutate ref: %s", err)
	}

	return fmt.Sprintf("Ref %s deleted", refId), nil
}

func main() {
	client, err := InitClient()
	if err != nil {
		fmt.Printf("could not initialize client: %s", err)
	}

	branchProtectionRuleID, err := client.GetBranchProtectionRuleID("release/")

	if err != nil {
		fmt.Printf("could not find branch protection rule: %s", err)
	}

	status, err := client.AllowDeleteProtectedBranch(branchProtectionRuleID, githubv4.Boolean(false))

	if err != nil {
		fmt.Printf("could not modify the branch protection rule: %s", err)
	}

	fmt.Print(status)

	// branch, err := client.GetBranches("release/", 5)
	// if err != nil {
	// 	fmt.Printf("could not list branches: %s", err)
	// }

}
