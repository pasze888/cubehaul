// Package output renders results as aligned tables or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"cubehaul/internal/platform"
)

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "json encode:", err)
	}
}

// Projects prints search results as a table or JSON.
func Projects(projects []platform.Project, asJSON bool) {
	if asJSON {
		printJSON(projects)
		return
	}
	if len(projects) == 0 {
		fmt.Println("no results")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tID\tSLUG\tTITLE\tAUTHOR\tDOWNLOADS\tFOLLOWS\tUPDATED")
	for _, p := range projects {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(p.Platform), p.ID, p.Slug,
			truncate(p.Title, 40), truncate(p.Author, 20),
			HumanCount(p.Downloads), HumanCount(p.Follows), shortDate(p.UpdatedAt))
	}
	w.Flush()
}

// Project prints a single project as a detail table or JSON.
func Project(p *platform.Project, asJSON bool) {
	if asJSON {
		printJSON(p)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Title:\t%s\n", p.Title)
	fmt.Fprintf(w, "Slug:\t%s\n", p.Slug)
	fmt.Fprintf(w, "ID:\t%s\n", p.ID)
	fmt.Fprintf(w, "Platform:\t%s\n", p.Platform)
	fmt.Fprintf(w, "Author:\t%s\n", orDash(p.Author))
	fmt.Fprintf(w, "Downloads:\t%d\n", p.Downloads)
	fmt.Fprintf(w, "Followers:\t%d\n", p.Follows)
	fmt.Fprintf(w, "License:\t%s\n", orDash(p.License))
	fmt.Fprintf(w, "Categories:\t%s\n", strings.Join(p.Categories, ", "))
	fmt.Fprintf(w, "Updated:\t%s\n", orDash(p.UpdatedAt))
	fmt.Fprintf(w, "URL:\t%s\n", p.URL)
	fmt.Fprintf(w, "Description:\t%s\n", wrap(p.Description, 100))
	w.Flush()
}

// Versions prints a version list as a table or JSON.
func Versions(versions []platform.Version, asJSON bool) {
	if asJSON {
		printJSON(versions)
		return
	}
	if len(versions) == 0 {
		fmt.Println("no versions")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tGAME VERSIONS\tLOADERS\tDATE")
	for _, v := range versions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			v.ID, truncate(v.VersionNumber, 30),
			truncate(strings.Join(v.GameVersions, ", "), 40),
			truncate(strings.Join(v.Loaders, ", "), 30),
			shortDate(v.DatePublished))
	}
	w.Flush()
}

// Categories prints a category tree as a table or JSON.
func Categories(cats []platform.Category, asJSON bool) {
	if asJSON {
		printJSON(cats)
		return
	}
	if len(cats) == 0 {
		fmt.Println("no categories")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSLUG")
	children := map[int][]platform.Category{}
	var roots []platform.Category
	for _, c := range cats {
		id, err := strconv.Atoi(c.ID)
		if err != nil {
			id = 0
		}
		if c.ParentID == 0 || c.IsClass {
			roots = append(roots, c)
			continue
		}
		_ = id
		children[c.ParentID] = append(children[c.ParentID], c)
	}
	var walk func(c platform.Category, depth int)
	walk = func(c platform.Category, depth int) {
		id, err := strconv.Atoi(c.ID)
		if err != nil {
			id = 0
		}
		fmt.Fprintf(w, "%s%s\t%s\t%s\n", strings.Repeat("  ", depth), c.ID, c.Name, c.Slug)
		for _, child := range children[id] {
			walk(child, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	w.Flush()
}

// HumanCount formats counts compactly, e.g. 1234567 -> "1.2M".
func HumanCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// HumanSize formats byte counts, e.g. 3456789 -> "3.3MB".
func HumanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func shortDate(s string) string {
	if s == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// wrap breaks s into lines no wider than width.
func wrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return "-"
	}
	var b strings.Builder
	line := 0
	for i, word := range words {
		if i > 0 && line+len(word)+1 > width {
			b.WriteString("\n")
			line = 0
		} else if i > 0 {
			b.WriteString(" ")
			line++
		}
		b.WriteString(word)
		line += len(word)
	}
	return b.String()
}
