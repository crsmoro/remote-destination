import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('position_adapter', Path(__file__).parents[1] / 'tools' / 'ha_position_adapter.py')
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)

class PositionAdapterTests(unittest.TestCase):
    def test_minimal_snapshot_and_unavailable(self):
        data = {'state':'home','attributes':{'latitude':-26.3,'longitude':-48.8,'vin':'sensitive'}}
        self.assertEqual(adapter.snapshot(data), {'available':True,'latitude':-26.3,'longitude':-48.8})
        self.assertEqual(adapter.snapshot({'state':'unavailable','attributes':data['attributes']}), {'available':False})
        self.assertEqual(adapter.snapshot({'state':'home','attributes':{'latitude':True,'longitude':1}}), {'available':False})

if __name__ == '__main__': unittest.main()
