"""Budget refusal must happen before creating any disposable Docker resources."""
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

source = Path(__file__).resolve().parents[1] / 'scripts/media-release.py'
spec = importlib.util.spec_from_file_location('media_release_fixture', source)
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class ReleaseBudget(unittest.TestCase):
    def test_insufficient_memory_or_cpu_creates_no_resources(self):
        for override in ({'envelope_mib': 1023}, {'envelope_cpus': 1.9}):
            values = dict(api_mib=256, worker_mib=512, reserve_mib=256, envelope_mib=1024,
                          api_cpus=1, worker_cpus=1, envelope_cpus=2)
            with self.subTest(override=override), patch.object(fixture, 'run') as docker:
                with self.assertRaisesRegex(RuntimeError, 'insufficient'):
                    fixture.fixture('colocated', SimpleNamespace(**{**values, **override}))
                docker.assert_not_called()


if __name__ == '__main__':
    unittest.main()
