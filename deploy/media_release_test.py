"""Budget refusal must happen before creating any disposable Docker resources."""
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, call, patch

source = Path(__file__).resolve().parents[1] / 'scripts/media-release.py'
spec = importlib.util.spec_from_file_location('media_release_fixture', source)
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class ReleaseBudget(unittest.TestCase):
    def test_resource_probe_retains_exact_kernel_counters_without_api_test_boot(self):
        report = {'memory_peak': 268435456, 'disk_bytes': 12345}
        with patch.object(fixture, 'run', return_value='RESOURCE_REPORT ' + fixture.json.dumps(report)) as docker:
            self.assertEqual(fixture.resource_snapshot('release-api'), report)
            docker.assert_called_once_with('exec', 'release-api', '/media-resource')

    def test_failed_resource_probe_is_not_counted_as_a_successful_sample(self):
        with patch.object(fixture, 'run', side_effect=RuntimeError('probe unavailable')):
            with self.assertRaisesRegex(RuntimeError, 'probe unavailable'):
                fixture.resource_snapshot('release-api')

        for report in ({}, {'memory_peak': 0, 'disk_bytes': 0},
                       {'memory_peak': True, 'disk_bytes': 0},
                       {'memory_peak': 123, 'disk_bytes': -1}):
            with self.subTest(report=report), patch.object(fixture, 'run', return_value='RESOURCE_REPORT ' + fixture.json.dumps(report)):
                with self.assertRaisesRegex(RuntimeError, 'invalid resource evidence'):
                    fixture.resource_snapshot('release-api')
        with patch.object(fixture, 'run', return_value='no kernel evidence'):
            with self.assertRaisesRegex(RuntimeError, 'resource evidence missing'):
                fixture.resource_snapshot('release-api')

    def test_clean_exit_between_state_check_and_probe_does_not_invent_a_sample(self):
        state = {'Running': False, 'ExitCode': 0, 'OOMKilled': False}
        with patch.object(fixture, 'run', side_effect=[RuntimeError('container is not running'),
                                                     fixture.json.dumps([{'State': state}])]) as docker:
            self.assertIsNone(fixture.resource_snapshot('release-api'))
            self.assertEqual(docker.call_args_list, [call('exec', 'release-api', '/media-resource'),
                                                    call('inspect', 'release-api')])

    def test_stopped_oom_failed_or_running_container_does_not_hide_probe_failure(self):
        for state in ({'Running': False, 'ExitCode': 137, 'OOMKilled': True},
                      {'Running': False, 'ExitCode': 1, 'OOMKilled': False},
                      {'Running': True, 'ExitCode': 0, 'OOMKilled': False}):
            with self.subTest(state=state), patch.object(fixture, 'run', side_effect=[
                    RuntimeError('container is not running'), fixture.json.dumps([{'State': state}])]):
                with self.assertRaisesRegex(RuntimeError, 'container is not running'):
                    fixture.resource_snapshot('release-api')

    def test_insufficient_memory_or_cpu_creates_no_resources(self):
        for override in ({'envelope_mib': 1023}, {'envelope_cpus': 1.9}):
            values = dict(api_mib=256, worker_mib=512, reserve_mib=256, envelope_mib=1024,
                          api_cpus=1, worker_cpus=1, envelope_cpus=2)
            with self.subTest(override=override), patch.object(fixture, 'run') as docker:
                with self.assertRaisesRegex(RuntimeError, 'insufficient'):
                    fixture.fixture('colocated', SimpleNamespace(**{**values, **override}))
                docker.assert_not_called()

    def test_invalid_budget_precedes_all_image_builds(self):
        with patch('sys.argv', ['media-release.py', '--envelope-mib', '1023']), patch.object(fixture, 'build') as build:
            with self.assertRaisesRegex(RuntimeError, 'insufficient'):
                fixture.main()
            build.assert_not_called()

    def test_storage_failure_precedes_application_builds_and_resources(self):
        with patch('sys.argv', ['media-release.py']), patch.object(fixture, 'build', side_effect=RuntimeError('source unavailable')) as build, patch.object(fixture, 'fixture') as release:
            with self.assertRaisesRegex(RuntimeError, 'source unavailable'):
                fixture.main()
            self.assertEqual(build.call_count, 1)
            self.assertEqual(build.call_args.args[0], 'media-storage')
            release.assert_not_called()

    def test_storage_precedes_both_layouts_even_when_application_builds_are_skipped(self):
        for skip in (False, True):
            events = Mock()
            with self.subTest(skip=skip), patch('sys.argv', ['media-release.py'] + (['--skip-build'] if skip else [])), patch.object(fixture, 'build', events.build), patch.object(fixture, 'fixture', events.release):
                fixture.main()
                self.assertEqual(events.mock_calls[0], call.build('media-storage', fixture.STORAGE_IMAGE, 'deploy/media/fixture.Dockerfile'))
                self.assertEqual(events.build.call_count, 1 if skip else 3)
                self.assertEqual([c.args[0] for c in events.release.call_args_list], ['colocated', 'remote'])

    def test_prebuilt_workflow_images_run_without_any_build(self):
        with patch('sys.argv', ['media-release.py', '--skip-build', '--skip-storage-build']), patch.object(fixture, 'build') as build, patch.object(fixture, 'run') as docker, patch.object(fixture, 'fixture') as release:
            fixture.main()
            build.assert_not_called()
            docker.assert_called_once_with('image', 'inspect', fixture.STORAGE_IMAGE)
            self.assertEqual([c.args[0] for c in release.call_args_list], ['colocated', 'remote'])

    def test_missing_prebuilt_storage_fails_before_resources_or_other_builds(self):
        with patch('sys.argv', ['media-release.py', '--skip-storage-build']), patch.object(fixture, 'build') as build, patch.object(fixture, 'run', side_effect=RuntimeError('image missing')), patch.object(fixture, 'fixture') as release:
            with self.assertRaisesRegex(RuntimeError, 'image missing'):
                fixture.main()
            build.assert_not_called()
            release.assert_not_called()


if __name__ == '__main__':
    unittest.main()
