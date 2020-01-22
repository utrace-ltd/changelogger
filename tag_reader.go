package changelogger

import (
	"fmt"
	chglog "github.com/git-chglog/git-chglog"
	"regexp"
	"sort"
	"strings"
	"time"

	gitcmd "github.com/tsuyoshiwada/go-gitcmd"
)

type tagReader struct {
	client    gitcmd.Client
	format    string
	separator string
	reFilter  *regexp.Regexp
}

func newTagReader(client gitcmd.Client, filterPattern string) *tagReader {
	return &tagReader{
		client:    client,
		separator: "@@__CHGLOG__@@",
		reFilter:  regexp.MustCompile(filterPattern),
	}
}

func (r *tagReader) ReadAll() ([]*chglog.Tag, error) {
	out, err := r.client.Exec(
		"for-each-ref",
		"--format",
		"%(refname)"+r.separator+"%(subject)"+r.separator+"%(taggerdate)"+r.separator+"%(authordate)",
		"refs/tags",
	)

	tags := []*chglog.Tag{}

	if err != nil {
		return tags, fmt.Errorf("failed to get git-tag: %s", err.Error())
	}

	lines := strings.Split(out, "\n")

	for _, line := range lines {
		tokens := strings.Split(line, r.separator)

		if len(tokens) != 4 {
			continue
		}

		name := r.parseRefname(tokens[0])
		subject := r.parseSubject(tokens[1])
		date, err := r.parseDate(tokens[2])
		if err != nil {
			t, err2 := r.parseDate(tokens[3])
			if err2 != nil {
				return nil, err2
			}
			date = t
		}

		if r.reFilter != nil {
			if !r.reFilter.MatchString(name) {
				continue
			}
		}

		tags = append(tags, &chglog.Tag{
			Name:    name,
			Subject: subject,
			Date:    date,
		})
	}

	r.sortTags(tags)
	r.assignPreviousAndNextTag(tags)

	return tags, nil
}

func (*tagReader) parseRefname(input string) string {
	return strings.Replace(input, "refs/tags/", "", 1)
}

func (*tagReader) parseSubject(input string) string {
	return strings.TrimSpace(input)
}

func (*tagReader) parseDate(input string) (time.Time, error) {
	return time.Parse("Mon Jan 2 15:04:05 2006 -0700", input)
}

func (*tagReader) assignPreviousAndNextTag(tags []*chglog.Tag) {
	total := len(tags)

	for i, tag := range tags {
		var (
			next *chglog.RelateTag
			prev *chglog.RelateTag
		)

		if i > 0 {
			next = &chglog.RelateTag{
				Name:    tags[i-1].Name,
				Subject: tags[i-1].Subject,
				Date:    tags[i-1].Date,
			}
		}

		if i+1 < total {
			prev = &chglog.RelateTag{
				Name:    tags[i+1].Name,
				Subject: tags[i+1].Subject,
				Date:    tags[i+1].Date,
			}
		}

		tag.Next = next
		tag.Previous = prev
	}
}

func (*tagReader) sortTags(tags []*chglog.Tag) {
	sort.Slice(tags, func(i, j int) bool {
		//return !tags[i].Date.Before(tags[j].Date)
		return VersionOrdinal(tags[i].Name) >  VersionOrdinal(tags[j].Name)
	})
}

func VersionOrdinal(version string) string {
	// ISO/IEC 14651:2011
	const maxByte = 1<<8 - 1
	vo := make([]byte, 0, len(version)+8)
	j := -1
	for i := 0; i < len(version); i++ {
		b := version[i]
		if '0' > b || b > '9' {
			vo = append(vo, b)
			j = -1
			continue
		}
		if j == -1 {
			vo = append(vo, 0x00)
			j = len(vo) - 1
		}
		if vo[j] == 1 && vo[j+1] == '0' {
			vo[j+1] = b
			continue
		}
		if vo[j]+1 > maxByte {
			panic("VersionOrdinal: invalid version")
		}
		vo = append(vo, b)
		vo[j]++
	}
	return string(vo)
}
