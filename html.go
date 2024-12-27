package main

import (
	"fmt"
	"io"
	"os"
	"slices"

	"google.golang.org/appengine/v2/user"
)

func renderPremable(w io.Writer, u *user.User, q string) {
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1.0">
		<title>PDA2GO</title>
		<style type="text/css">
			input[type=text],textarea {
				background-color: #eff9fb;
			}
			body {
				line-height: 1.3em;
				background-color: #e7f4ff;
			}
			.comments {
				font-family: monospace;
				color: #c44;
				white-space: pre;
				padding-bottom: 2em;
			}
			.tag {
				font-size: small;
			}
			.indent {
				padding-left: 2em;
			}
			.edit-link {
				font-size: small;
				background-color: #ddd;
				margin: 0.2em;
				padding: 0px 10px;
				border-radius: 5px;
				display: inline-block;
			}
			.thing {
				font-weight: bold;
				margin: 0em 0.5em 0em 0.2em;
			}
			.thing.Address {
				color: green;
			}
			.thing.Contact {
				color: blue;
			}
			.thing.Calendar {
				color: purple;
			}
			.title {
				text-decoration: none;
				font-size: 2em;
				font-weight: bold;
				padding: 0.2em 0em 0.5em;
				display: block;
				color: black;
			}
			.title .go {
				color: darkgreen;
			}
			.powered {
				color: #777;
				font-style: italic;
				margin-left: 42%%;
			}
			.email {
				position: absolute;
				right: 0.5em;
				top: 0.2em;
				color: #444;
			}
			.disabled , .disabled * {
				color: #ccc !important;
			}
			a.admin, a:visited.admin {
				color: #f44;
			}
			.version {
				color: #800;
				font-family: monospace;
			}
		</style>
		</head>
		<body class="pda">
			<a href="/" class="title">PDA2<span class="go">GO</span></a></a>
			<div class="email">%s</div>
			<form name="searchform" method="get">
				<!--
				<input type="checkbox" name="includedisabled"> Include Disabled Entries<br>
				<br>
				<input type="radio" name="format" checked value="verbose"> Verbose (regular) results<br>
				<input type="radio" name="format"  value="compact"> Compact results<br>
				-->

				Search: <input type="text" name="q" value="%s"> <input type="submit" value="Go"><br>
			</form>

			<hr>
			[<a href=".?action=create&kind=Person">+entry</a>]
			<br>
			<br>
	`,
		u.Email,
		q)
}

func renderPostamble(w io.Writer, u *user.User, q string) {
	if isDev || slices.Contains(ADMINS_FREDSA, u.Email) {
		fmt.Fprintf(w, `
		<div><a class="admin" href="/_ah/admin">/_ah/admin</a></div>
		<div><a class="admin" href="/_ah/stats">/_ah/stats</a></div>
		<div><a class="admin" href="/task/notify">/task/notify</a></div>
		`)
	}

	fmt.Fprintf(w, `
	<script>
	document.searchform.q.focus();
	document.searchform.q.select();
	</script>
	`)

	fmt.Fprintf(w, `
		<div class="powered">
			<span class="version">%s</span>,
			powered by Go on App Engine
			(%s %s)
		</div>
	</body>
	</html>
    `,
		os.Getenv(GAE_VERSION),
		os.Getenv(GAE_ENV),
		os.Getenv(GAE_RUNTIME))
}
