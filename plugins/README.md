<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Alexander Hirsch <Hirsch@med.uni-frankfurt.de>

SPDX-License-Identifier: EUPL-1.2
-->

# lookup plugins

- [`parse_key_directory_to_users`](#parse_key_directory_to_users) generate a users-table from authorized_keys-style files

## `parse_key_directory_to_users`

The `parse_key_directory_to_users` lookup plugin generates a users-table from authorized_keys-style files in a directory.

Given a directory like this:
```
└── users
    ├── alice
    └── bob
```
you can create the users table for `mkbrechtel.sysops.users` via
```yaml
users: {{ lookup('mkbrechtel.sysops.parse_key_directory_to_users', 'users') }}
```
The generates users adopt the name of the respective file.

The plugin takes two optional parameters, `extension` and `attributes`.

`extension` specifies a file extension.  
This serves two purposes, to filter the files and to strip it from the username.

A directory like this:
```
└── users
    ├── README.md
    ├── alice.pub
    └── bob.pub
```
and the configuration like this:
```yaml
users: {{ lookup('mkbrechtel.sysops.parse_key_directory_to_users', 'users', '.pub') }}
```
will skip the `README.md` and create two users named "alice" and "bob".

With `attributes` a mapping can be passed to add some attributes to each user.
```yaml
users: {{ lookup(
  'mkbrechtel.sysops.parse_key_directory_to_users', 'users', '.pub',
  {
    shell: 'zsh',
    groups: ['adm'],
  }
) }}
```

If you want to change properties for individual users you will need to merge the parsed mapping with the custom properties, like this for instance:
```yaml
users: {{
  lookup('mkbrechtel.sysops.parse_key_directory_to_users', 'users')
  | combine(
      {
        'alice': { 'shell': 'fish' },
      },
      recursive=true,
    )
}}
```
