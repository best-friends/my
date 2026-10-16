// SPDX-License-Identifier: AGPL-3.0-only

package content

// StatusURI returns the ActivityPub id of a local status on host (ADR-0018).
func StatusURI(host, statusID string) string {
	return "https://" + host + "/ap/objects/" + statusID
}

// StatusURL returns the HTML page of a local status on host (ADR-0018). The page is found by
// the status ID alone; username only makes the URL readable.
func StatusURL(host, username, statusID string) string {
	return "https://" + host + "/@" + username + "/" + statusID
}
