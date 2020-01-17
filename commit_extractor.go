package changelogger

import (
	"fmt"
	chglog "github.com/git-chglog/git-chglog"
	"sort"
	"strings"
)

type commitExtractor struct {
	opts *chglog.Options
}

func newCommitExtractor(opts *chglog.Options) *commitExtractor {
	return &commitExtractor{
		opts: opts,
	}
}

func (e *commitExtractor) Extract(commits []*chglog.Commit) ([]*chglog.CommitGroup, []*chglog.Commit, []*chglog.Commit, []*chglog.NoteGroup) {
	commitGroups := []*chglog.CommitGroup{}
	noteGroups := []*chglog.NoteGroup{}
	mergeCommits := []*chglog.Commit{}
	revertCommits := []*chglog.Commit{}

	filteredCommits := commitFilter(commits, e.opts.CommitFilters)

	for _, commit := range commits {
		if commit.Merge != nil {
			mergeCommits = append(mergeCommits, commit)
			continue
		}

		if commit.Revert != nil {
			revertCommits = append(revertCommits, commit)
			continue
		}
	}

	for _, commit := range filteredCommits {
		if commit.Merge == nil && commit.Revert == nil {
			e.processCommitGroups(&commitGroups, commit)
		}

		e.processNoteGroups(&noteGroups, commit)
	}

	e.sortCommitGroups(commitGroups)
	e.sortNoteGroups(noteGroups)

	return commitGroups, mergeCommits, revertCommits, noteGroups
}

func (e *commitExtractor) processCommitGroups(groups *[]*chglog.CommitGroup, commit *chglog.Commit) {
	var group *chglog.CommitGroup

	// commit group
	raw, ttl := e.commitGroupTitle(commit)

	for _, g := range *groups {
		if g.RawTitle == raw {
			group = g
		}
	}

	if group != nil {
		group.Commits = append(group.Commits, commit)
	} else if raw != "" {
		*groups = append(*groups, &chglog.CommitGroup{
			RawTitle: raw,
			Title:    ttl,
			Commits:  []*chglog.Commit{commit},
		})
	}
}

func (e *commitExtractor) processNoteGroups(groups *[]*chglog.NoteGroup, commit *chglog.Commit) {
	if len(commit.Notes) != 0 {
		for _, note := range commit.Notes {
			e.appendNoteToNoteGroups(groups, note)
		}
	}
}

func (e *commitExtractor) appendNoteToNoteGroups(groups *[]*chglog.NoteGroup, note *chglog.Note) {
	exist := false

	for _, g := range *groups {
		if g.Title == note.Title {
			exist = true
			g.Notes = append(g.Notes, note)
		}
	}

	if !exist {
		*groups = append(*groups, &chglog.NoteGroup{
			Title: note.Title,
			Notes: []*chglog.Note{note},
		})
	}
}

func (e *commitExtractor) commitGroupTitle(commit *chglog.Commit) (string, string) {
	var (
		raw string
		ttl string
	)

	if title, ok := getTypeByCommit(commit, e.opts.CommitGroupBy); ok {
		if ok == true {
			raw = title
			if t, ok := e.opts.CommitGroupTitleMaps[title]; ok {
				ttl = t
			} else {
				ttl = strings.Title(raw)
			}
		}
	}

	return raw, ttl
}

func (e *commitExtractor) sortCommitGroups(groups []*chglog.CommitGroup) {
	// groups
	sort.Slice(groups, func(i, j int) bool {
		var (
			a, b interface{}
			ok   bool
		)

		a, ok = dotGet(groups[i], e.opts.CommitGroupSortBy)
		if !ok {
			return false
		}

		b, ok = dotGet(groups[j], e.opts.CommitGroupSortBy)
		if !ok {
			return false
		}

		res, err := compare(a, "<", b)
		if err != nil {
			return false
		}
		return res
	})

	// commits
	for _, group := range groups {
		sort.Slice(group.Commits, func(i, j int) bool {
			var (
				a, b interface{}
				ok   bool
			)

			a, ok = dotGet(group.Commits[i], e.opts.CommitSortBy)
			if !ok {
				return false
			}

			b, ok = dotGet(group.Commits[j], e.opts.CommitSortBy)
			if !ok {
				return false
			}

			res, err := compare(a, "<", b)
			if err != nil {
				return false
			}
			return res
		})
	}
}

func (e *commitExtractor) sortNoteGroups(groups []*chglog.NoteGroup) {
	// groups
	sort.Slice(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Title) < strings.ToLower(groups[j].Title)
	})

	// notes
	for _, group := range groups {
		sort.Slice(group.Notes, func(i, j int) bool {
			return strings.ToLower(group.Notes[i].Title) < strings.ToLower(group.Notes[j].Title)
		})
	}
}

func getTypeByCommit(commit *chglog.Commit, prop string) (string, bool) {
	path := strings.Split(prop, ".")

	if len(path) == 0 {
		return "", false
	}

	for _, key := range path {
		if strings.LastIndex(commit.Header, fmt.Sprintf("(%s)", key)) > 0 {
			return key, true
		}
	}

	return "", false
}