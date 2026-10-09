# Issue tracker: GitHub

Issues and specs live in GitHub Issues for `nanfxqs/campus-market`.
Use the `gh` CLI from this clone, or specify
`--repo nanfxqs/campus-market`.

## Conventions

- Create: `gh issue create --title "..." --body-file <file>`.
- Read: `gh issue view <number> --json number,title,body,labels,comments`.
- List: `gh issue list --state open --json number,title,body,labels,comments`.
  Add label and state filters as needed.
- Comment: `gh issue comment <number> --body-file <file>`.
- Apply or remove labels: `gh issue edit <number> --add-label "..."`
  or `--remove-label "..."`.
- Close: `gh issue close <number> --comment "..."`.

Write multiline bodies to a temporary file and pass `--body-file`.
For child tickets, put `Part of #<parent>` at the top of the body
and add a task-list link in the parent issue.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## Skill operations

When a skill says "publish to the issue tracker", create a GitHub issue.
When it says "fetch the relevant ticket", read the corresponding issue.
