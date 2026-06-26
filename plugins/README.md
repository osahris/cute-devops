<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Alexander Hirsch <Hirsch@med.uni-frankfurt.de>

SPDX-License-Identifier: EUPL-1.2
-->

# Collections Plugins Directory

This directory can be used to ship various plugins inside an Ansible collection. Each plugin is placed in a folder that
is named after the type of plugin it is in. It can also include the `module_utils` and `modules` directory that
would contain module utils and modules respectively.

Here is an example directory of the majority of plugins currently supported by Ansible:

```
└── plugins
    ├── action
    ├── become
    ├── cache
    ├── callback
    ├── cliconf
    ├── connection
    ├── filter
    ├── httpapi
    ├── inventory
    ├── lookup
    ├── module_utils
    ├── modules
    ├── netconf
    ├── shell
    ├── strategy
    ├── terminal
    ├── test
    └── vars
```

A full list of plugin types can be found at [Working With Plugins](https://docs.ansible.com/ansible-core/devel/plugins/plugins.html).

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
