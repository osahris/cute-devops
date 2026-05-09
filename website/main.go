// SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"io/fs"
	"net/http"
)

func main() {
	siteFS, err := fs.Sub(SiteFiles, "dist")
	if err != nil {
		fmt.Printf("Failed to mount embedded site: %v\n", err)
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(siteFS)))

	fmt.Println("Starting server on :4780")
	if err := http.ListenAndServe(":4780", mux); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
