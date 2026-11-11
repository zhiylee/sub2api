"""Downstream publication policy; keep the upstream build pipeline intact."""

import re

VERSION_RE = re.compile(r'(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)-zy\.[1-9][0-9]*')


def validate_mode(version, simple):
    if VERSION_RE.fullmatch(version) and simple:
        raise ValueError('downstream online updates require a full release; simple mode has no binary assets')


def configure_publication(data, version, simple):
    validate_mode(version, simple)
    if VERSION_RE.fullmatch(version):
        data['release']['prerelease'] = False
        data['release']['make_latest'] = True
        data['release']['footer'] = data['release']['footer'].replace(
            '/main/deploy/install.sh', f'/v{version}/deploy/install.sh'
        )
