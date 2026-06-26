#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Alexander Hirsch <Hirsch@med.uni-frankfurt.de>
#
# SPDX-License-Identifier: EUPL-1.2

from ansible.errors import AnsibleError
from ansible.plugins.lookup import LookupBase
from ansible.utils.display import Display
import os

display = Display()

DOCUMENTATION = """
    name: parse_key_directory_to_users
    short_description: Generate mkbrechtel.sysops.users from authorized_keys-style files
    description:
        - Read authorized_keys-style files from a directory.
        - The filename dictates the username.
    notes:
        - The directory is not traversed recursively.
    positional: _path, _extension, _attributes
    options:
      _path:
        description: directory containing the files to parse
        required: True
      _extension:
        description: file extensions, stripped when determining the username
        default: ''
      _attributes:
        description: additional attributes to insert for every user
        default: {}
"""

class LookupModule(LookupBase):
    @staticmethod
    def run(args, variables=None):
        try:
            assert 1 <= len(args) <= 3, f"got {len(args)} arguments, expected 1-3"
            path, extension, attributes = args + ['', {}][len(args) - 1:]
            assert isinstance(path, str), f"path is of type {type(path)}, expected str"
            assert isinstance(extension, str), f"extension is of type {type(extension)}, expected str"
            assert isinstance(attributes, dict), f"attributes is of type {type(attributes)}, expected dict"
        except Exception as e:
            raise AnsibleError(f"Invalid arguments: {e}")

        users = {}
        for filename in sorted(os.listdir(path)):
            filepath = os.path.join(path, filename)
            if not filename.endswith(extension):
                display.vv(f"Discarding file '{filepath}' (does not end in {extension})")
                continue

            display.v(f"Reading file: '{filepath}'")
            try:
                username = filename[:-len(extension)]
                assert username not in users
                users[username] = {
                    'ssh_authorized_keys': list(
                        filter(
                            # remove empty lines
                            lambda line: line,
                            map(str.strip, open(filepath).readlines())
                        )
                    ),
                    **attributes
                }
            except Exception as e:
                raise AnsibleError(e)

        # lookups must return a list
        # single-element lists are flattened by ansible
        return [users]
