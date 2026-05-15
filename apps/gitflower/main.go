// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"os"

	"gitflower/app"
)

func main() {
	os.Exit(app.App(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
