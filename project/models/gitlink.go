package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Git integration: links an issue to external code artifacts — commits,
// branches and pull requests — so the board shows delivery status like Jira's
// dev panel. The link stores references, not code: providers sync via the
// object actions or API writes.

const gitLinkModel = "project.git.link"

type GitLinkKind string

const (
	GitCommit GitLinkKind = "commit"
	GitBranch GitLinkKind = "branch"
	GitPR     GitLinkKind = "pull_request"
)

type GitLinkState string

const (
	GitOpen   GitLinkState = "open"
	GitMerged GitLinkState = "merged"
	GitClosed GitLinkState = "closed"
	GitDraft  GitLinkState = "draft"
)

// ProjectGitLink ties one issue to one code artifact.
type ProjectGitLink struct {
	sdk.Model `nexa:"model=project.git.link"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	TaskID    sdk.Many2One[ProjectTask]    `nexa:"required,index,ondelete=cascade,string=Issue"`
	ProjectID sdk.Many2One[ProjectProject] `nexa:"index,string=Project"`
	Kind      sdk.Selection[GitLinkKind]   `nexa:"required,string=Kind,default=commit,selection=commit:Commit,branch:Branch,pull_request:Pull Request"`
	Repo      sdk.String                   `nexa:"string=Repository"`
	Ref       sdk.String                   `nexa:"required,string=Reference,help='Commit SHA, branch name, or PR number'"`
	Title     sdk.String                   `nexa:"string=Title"`
	URL       sdk.String                   `nexa:"string=URL"`
	Author    sdk.String                   `nexa:"string=Author"`
	State     sdk.Selection[GitLinkState]  `nexa:"string=Status,default=open,selection=open:Open,draft:Draft,merged:Merged,closed:Closed"`
	LinkedAt  sdk.Date                     `nexa:"string=Date"`
}

func init() {
	orm.RegisterConstraint(gitLinkModel, "git_link_unique", constraintGitLinkUnique)
	orm.RegisterObjectAction(gitLinkModel, "action_ingest_commit", gitIngestCommit)
}

// gitIngestCommit is the provider-agnostic sync entry point. Call it through
// the JSON-RPC action endpoint (or a future webhook bridge) with args:
//
//	repo, ref (sha/branch/pr), kind, title, url, author, state, message
//
// It scans `message` for issue keys (PROJECT-123 style, matching each project's
// code prefix) and creates/updates git.link rows on the matched issues. Returns
// the number of issues linked. Duplicate (task, kind, ref) rows are skipped.
func gitIngestCommit(ctx context.Context, _ string, _ int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	repo := strings.TrimSpace(args["repo"])
	ref := strings.TrimSpace(args["ref"])
	if repo == "" || ref == "" {
		return "", fmt.Errorf("repo and ref are required")
	}
	kind := GitLinkKind(strings.TrimSpace(args["kind"]))
	if kind == "" {
		kind = GitCommit
	}
	state := GitLinkState(strings.TrimSpace(args["state"]))
	if state == "" {
		state = GitOpen
	}
	message := args["message"] + " " + args["title"]

	// Collect project code prefixes → project id for key matching.
	projs, err := orm.SearchPage(bypass, projModel, nil, 500, 0, "id asc")
	if err != nil {
		return "", err
	}
	prefixes := map[string]int{}
	for _, p := range projs {
		code := strings.ToUpper(strings.TrimSpace(str(p["code"])))
		if code != "" {
			prefixes[code] = intID(p["id"])
		}
	}

	linked := 0
	seen := map[int]bool{}
	upper := strings.ToUpper(message)
	for prefix, pid := range prefixes {
		// match PREFIX-<digits> tokens
		idx := 0
		for {
			i := strings.Index(upper[idx:], prefix+"-")
			if i < 0 {
				break
			}
			i += idx
			rest := upper[i+len(prefix)+1:]
			j := 0
			for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
				j++
			}
			if j == 0 {
				idx = i + len(prefix) + 1
				continue
			}
			key := prefix + "-" + rest[:j]
			// find the task by key + project
			tasks, terr := orm.SearchPage(bypass, taskModel,
				[][]interface{}{{"key", "=", key}, {"project_id", "=", pid}}, 1, 0, "id asc")
			if terr == nil && len(tasks) > 0 {
				tid := intID(tasks[0]["id"])
				if !seen[tid] {
					seen[tid] = true
					if gitLinkUpsert(bypass, tid, pid, kind, repo, ref, state, args) {
						linked++
					}
				}
			}
			idx = i + len(prefix) + 1 + j
		}
	}
	return fmt.Sprintf("linked %d issue(s)", linked), nil
}

// gitLinkUpsert creates the link row unless an identical (task, kind, ref)
// already exists — in which case it refreshes state/title/url.
func gitLinkUpsert(ctx context.Context, taskID, projectID int, kind GitLinkKind, repo, ref string, state GitLinkState, args map[string]string) bool {
	existing, err := orm.SearchPage(ctx, gitLinkModel, [][]interface{}{
		{"task_id", "=", taskID}, {"kind", "=", string(kind)}, {"ref", "=", ref},
	}, 1, 0, "id asc")
	vals := map[string]interface{}{
		"task_id": taskID, "project_id": projectID, "kind": string(kind),
		"repo": repo, "ref": ref, "state": string(state),
		"title": args["title"], "url": args["url"], "author": args["author"],
	}
	if err == nil && len(existing) > 0 {
		_ = orm.UpdateRecordByID(ctx, gitLinkModel, intID(existing[0]["id"]), vals)
		return false
	}
	inst, ok := orm.Registry[gitLinkModel]
	if !ok {
		return false
	}
	_, err = orm.Create(ctx, inst, vals)
	return err == nil
}

// constraintGitLinkUnique refuses duplicate (task, kind, ref) rows — re-linking
// the same commit updates state/title instead of stacking copies.
func constraintGitLinkUnique(ctx context.Context, rec map[string]interface{}) error {
	tid, kind, ref := intID(rec["task_id"]), str(rec["kind"]), strings.TrimSpace(str(rec["ref"]))
	if tid <= 0 || kind == "" || ref == "" {
		return nil
	}
	id := intID(rec["id"])
	rows, err := orm.SearchPage(ctx, gitLinkModel, [][]interface{}{
		{"task_id", "=", tid},
		{"kind", "=", kind},
		{"ref", "=", ref},
	}, 10, 0, "id asc")
	if err != nil {
		return nil
	}
	for _, row := range rows {
		if rid := intID(row["id"]); rid != id && rid > 0 {
			return fmt.Errorf("this issue already links to %s %s", kind, ref)
		}
	}
	return nil
}
