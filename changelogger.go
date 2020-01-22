package changelogger

import (
	"errors"
	"fmt"
	chglog "github.com/git-chglog/git-chglog"
	"github.com/tsuyoshiwada/go-gitcmd"
)

type ChangeLogger struct {
	config *chglog.Config
	client *gitcmd.Client
	tagSelector *tagSelector
	tagReader *tagReader
	commitParser *commitParser
	commitExtractor *commitExtractor
}

func NewChangeLogger() *ChangeLogger {
	config := &chglog.Config{
		Options:&chglog.Options{
			RefActions: make([]string, 0),
			MergePattern: "Merge branch",
			RevertPattern: "Revert",
			CommitGroupBy: "bugfix.feat.hotfix.fix.feature",
			CommitGroupTitleMaps: map[string]string {
				"fix": "Bugfix",
				"feat": "Feature",
			},
		},
	}

	client := gitcmd.New(&gitcmd.Config{
		Bin: config.Bin,
	})

	tagReader := newTagReader(client, "")
	tagSelector := newTagSelector()
	commitParser := newCommitParser(client, config)
	commitExtractor := newCommitExtractor(config.Options)

	return &ChangeLogger{config:config, client:&client, commitParser:commitParser, commitExtractor:commitExtractor, tagReader:tagReader, tagSelector:tagSelector}
}

func (c *ChangeLogger) GetVersionChangeLog(query string) (*chglog.Version, error) {
	v, err := c.GetChangeLog(query)
	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, errors.New(fmt.Sprintf("Version %s not found", query))
	}

	return v[0], nil
}

func (c *ChangeLogger) GetChangeLog(query string) ([]*chglog.Version, error) {
	tags, first, err := c.getTags(query)
	if err != nil {
		return nil, err
	}

	versions, err := c.readVersions(tags, first)
	if err != nil {
		return nil, err
	}

	return versions, nil
}

func (c *ChangeLogger) getTags(query string) ([]*chglog.Tag, string, error) {
	tags, err := c.tagReader.ReadAll()
	if err != nil {
		return nil, "", err
	}

	if len(tags) == 0 {
		return nil, "", errors.New("git-tag does not exist")
	}

	first := ""
	if query != "" {
		tags, first, err = c.tagSelector.Select(tags, query)
		if err != nil {
			return nil, "", err
		}
	}

	return tags, first, nil
}

func (c *ChangeLogger)readVersions(tags []*chglog.Tag, first string) ([]*chglog.Version, error) {
	var versions []*chglog.Version

	for i, tag := range tags {
		var (
			rev    string
		)

		if i+1 < len(tags) {
			rev = tags[i+1].Name + ".." + tag.Name
		} else {
			if first != "" {
				rev = first + ".." + tag.Name
			} else {
				rev = tag.Name
			}
		}

		commits, err := c.commitParser.Parse(rev)
		if err != nil {
			return nil, err
		}

		commitGroups, mergeCommits, revertCommits, noteGroups := c.commitExtractor.Extract(commits)

		versions = append(versions, &chglog.Version{
			Tag:           tag,
			CommitGroups:  commitGroups,
			Commits:       commits,
			MergeCommits:  mergeCommits,
			RevertCommits: revertCommits,
			NoteGroups:    noteGroups,
		})
	}

	return versions, nil
}
