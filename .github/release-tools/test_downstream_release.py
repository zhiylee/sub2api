import argparse
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import yaml

import downstream_release
import release_matrix

ROOT = Path(__file__).resolve().parents[2]


class DownstreamReleaseTest(unittest.TestCase):
    def setUp(self):
        previous = Path.cwd()
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.addCleanup(os.chdir, previous)
        os.chdir(temp.name)
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            shutil.copyfile(ROOT / name, name)

    def test_only_stable_zy_revisions_are_promoted(self):
        for version in ('0.2.21-zy.1', '0.2.21-zy.10', '0.2.21', '0.2.22-rc.1', '0.2.21-zy.01'):
            with self.subTest(version=version), patch.dict(os.environ, {'RELEASE_VERSION': version}):
                release_matrix.generate_config(argparse.Namespace(mode='publish', simple=False, output='publisher.yaml'))
                data = yaml.safe_load(Path('publisher.yaml').read_text())
                if downstream_release.VERSION_RE.fullmatch(version):
                    self.assertIs(data['release']['prerelease'], False)
                    self.assertIs(data['release']['make_latest'], True)
                    self.assertEqual(data['release']['extra_files'], data['checksum']['extra_files'])
                    self.assertEqual(data['checksum']['name_template'], 'checksums.txt')
                    self.assertIn(f'/v{version}/deploy/install.sh', data['release']['footer'])
                    self.assertNotIn('/main/deploy/install.sh', data['release']['footer'])
                else:
                    self.assertEqual(data['release']['prerelease'], 'auto')
                    self.assertNotIn('make_latest', data['release'])
                    self.assertIn('/main/deploy/install.sh', data['release']['footer'])

    def test_simple_downstream_publication_fails_before_building(self):
        Path('backend/cmd/server').mkdir(parents=True)
        release_matrix.VERSION_FILE.write_text('0.2.21-zy.1\n')
        with patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            with self.assertRaisesRegex(ValueError, 'require a full release'):
                release_matrix.plan(argparse.Namespace(ref='v0.2.21-zy.2', dry_run=False, simple=True))
        self.assertEqual(release_matrix.VERSION_FILE.read_text(), '0.2.21-zy.1\n')
        with patch.dict(os.environ, {'RELEASE_VERSION': '0.2.21-zy.2'}):
            with self.assertRaisesRegex(ValueError, 'require a full release'):
                release_matrix.generate_config(argparse.Namespace(mode='publish', simple=True, output='publisher.yaml'))

    def test_version_file_sync_targets_the_integration_branch(self):
        workflow = yaml.safe_load((ROOT / '.github/workflows/release.yml').read_text())
        steps = workflow['jobs']['sync-version-file']['steps']
        selector = next(step['run'] for step in steps if step.get('id') == 'version_branch')
        for version, branch in [('0.2.21-zy.10', 'release/0.2.21'), ('0.2.22', 'main'), ('0.2.22-rc.1', 'main')]:
            output = Path('outputs').resolve()
            output.write_text('')
            subprocess.run(['bash', '-euo', 'pipefail', '-c', selector], check=True, env={
                **os.environ, 'RELEASE_VERSION': version, 'DEFAULT_BRANCH': 'main', 'GITHUB_OUTPUT': str(output)
            })
            self.assertEqual(output.read_text().strip(), f'branch={branch}')
        self.assertEqual(steps[-1]['run'].splitlines()[-1], 'git push origin HEAD:${{ steps.version_branch.outputs.branch }}')


if __name__ == '__main__':
    unittest.main()
