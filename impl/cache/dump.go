package cache

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/impl/serialize"
)

// digestPrefix separates the repository from the digest in a digest url.
const digestPrefix = "@sha256:"

// DumpCache returns the in-memory manifest cache as text: every entry, sorted by url, then
// every digest alias, sorted by url, with its tags. For each entry it shows the url, the url
// digest (the name of the manifest file), the manifest digest, and the manifest type. Digests,
// including the digest in a digest url, are shortened to digestLen hex characters followed by
// "..."; 64 or more shows them in full. mc is held while the cache is walked and the text is
// built, and is released before the caller sends the text, so a slow client can't hold it.
func DumpCache(digestLen int) string {
	mc.Lock()
	defer mc.Unlock()
	var sb strings.Builder
	sb.WriteString("ENTRIES:\n\n")
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ENTRY URL\tURL DIGEST\tMANIFEST DIGEST\tMANIFEST TYPE")
	for _, key := range slices.Sorted(maps.Keys(mc.entries)) {
		mh := mc.entries[key]
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", shortUrl(string(key), digestLen), shortDigest(serialize.UrlDigest(string(key)), digestLen),
			shortDigest(mh.Digest, digestLen), manifestType(mh))
	}
	tw.Flush()
	sb.WriteString("\nALIASES:\n\nALIAS URL / TAGS\n")
	for _, alias := range slices.Sorted(maps.Keys(mc.aliases)) {
		sb.WriteString(shortUrl(string(alias), digestLen) + "\n")
		for _, tag := range slices.Sorted(maps.Keys(mc.aliases[alias])) {
			sb.WriteString("    " + string(tag) + "\n")
		}
	}
	return sb.String()
}

// shortDigest returns the first n characters of the passed digest followed by "...", or the
// digest unchanged if it has n or fewer characters.
func shortDigest(digest string, n int) string {
	if len(digest) <= n {
		return digest
	}
	return digest[:n] + "..."
}

// shortUrl shortens the digest in the passed url with shortDigest. A tag url is returned
// unchanged.
func shortUrl(url string, n int) string {
	i := strings.LastIndex(url, digestPrefix)
	if i < 0 {
		return url
	}
	i += len(digestPrefix)
	return url[:i] + shortDigest(url[i:], n)
}

// manifestType returns "image list" for a manifest list or index, else "image".
func manifestType(mh imgpull.ManifestHolder) string {
	if mh.IsManifestList() {
		return "image list"
	}
	return "image"
}
