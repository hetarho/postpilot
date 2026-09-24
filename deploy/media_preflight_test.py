from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))
import media_preflight as preflight


class CapacityGate(unittest.TestCase):
    def test_sufficient_and_each_insufficient_resource(self):
        available = dict(available_memory_bytes=1024**3, available_disk_bytes=12*1024**3, cpu_capacity=2)
        def check(c):
            return preflight.assess(c,512*1024**2,256*1024**2,256*1024**2,8*1024**3,2*1024**3,1)
        self.assertTrue(check(available)['pass'])
        for key,value in [('available_memory_bytes',1024**3-1),('available_disk_bytes',10*1024**3-1),('cpu_capacity',.5)]:
            with self.subTest(resource=key):
                self.assertFalse(check({**available,key:value})['pass'])

    def test_memavailable_accounts_for_existing_workloads_and_cgroup(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            (root/'meminfo').write_text('MemTotal: 8388608 kB\nMemAvailable: 800000 kB\n')
            (root/'memory.max').write_text(str(1024**3))
            (root/'memory.current').write_text(str(512*1024**2))
            (root/'cpu.max').write_text('100000 100000')
            capacity=preflight.read_capacity(root,root,root)
            self.assertEqual(capacity['available_memory_bytes'],512*1024**2)
            self.assertEqual(capacity['cpu_capacity'],1)
            (root/'memory.max').write_text('max')
            self.assertEqual(preflight.read_capacity(root,root,root)['available_memory_bytes'],800000*1024)

    def test_zero_or_negative_budgets_fail(self):
        c=dict(available_memory_bytes=10**12,available_disk_bytes=10**12,cpu_capacity=8)
        self.assertFalse(preflight.assess(c,0,0,1,1,1,1)['pass'])
        self.assertFalse(preflight.assess(c,512*1024**2,-1,1,1,1,1)['pass'])

if __name__=='__main__':unittest.main()
