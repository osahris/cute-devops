// SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "embed"

//go:embed all:dist
var SiteFiles embed.FS
