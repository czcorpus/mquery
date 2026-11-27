package ui

import "strings"

const helpSep = " • "

// helpPlainPrefixes mark help segments which do not start with a key
var helpPlainPrefixes = []string{"type ", "any ", "~ "}

// renderHelp styles a footer help string of the form
// "key action • key action • ..." so that keys are highlighted
// and actions (and separators) are muted.
func renderHelp(help string) string {
	segments := strings.Split(help, helpSep)
	var b strings.Builder
	for i, seg := range segments {
		if i > 0 {
			b.WriteString(mutedStyle.Render(helpSep))
		}
		b.WriteString(renderHelpSegment(seg))
	}
	return b.String()
}

func renderHelpSegment(seg string) string {
	for _, p := range helpPlainPrefixes {
		if strings.HasPrefix(seg, p) {
			return mutedStyle.Render(seg)
		}
	}
	key, action, found := strings.Cut(seg, " ")
	if !found {
		return footerKeyStyle.Render(seg)
	}
	return footerKeyStyle.Render(key) + mutedStyle.Render(" "+action)
}
