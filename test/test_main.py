import copy
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import main


class ConfigTests(unittest.TestCase):
    def test_missing_config_is_created_for_editing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            with mock.patch.object(main, "script_dir", return_value=temp_dir):
                cfg, config_path, created = main.load_config()

            self.assertTrue(created)
            self.assertTrue(os.path.isfile(config_path))
            self.assertEqual(cfg["engine_path"], main.DEFAULTS["engine_path"])

    def test_load_config_fills_new_defaults(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = os.path.join(temp_dir, main.CONFIG_FILENAME)
            with open(config_path, "w", encoding="utf-8") as file:
                json.dump({"engine_path": __file__}, file)

            with mock.patch.object(main, "script_dir", return_value=temp_dir):
                cfg, loaded_path, created = main.load_config()

            self.assertFalse(created)
            self.assertEqual(loaded_path, config_path)
            self.assertEqual(cfg["allowed_origins"], ["https://xiangqiai.com"])
            self.assertEqual(cfg["max_connections"], 4)

    def test_validate_config_accepts_a_real_absolute_file(self):
        cfg = copy.deepcopy(main.DEFAULTS)
        cfg["engine_path"] = os.path.abspath(__file__)
        main.validate_config(cfg)

    def test_validate_config_rejects_unsafe_connection_count(self):
        cfg = copy.deepcopy(main.DEFAULTS)
        cfg["engine_path"] = os.path.abspath(__file__)
        cfg["max_connections"] = 0
        with self.assertRaisesRegex(ValueError, "max_connections"):
            main.validate_config(cfg)


class CommandTests(unittest.TestCase):
    def test_hash_command_is_rewritten_case_insensitively(self):
        old_hash = main.HASH
        try:
            main.HASH = 1024
            command, changed = main.rewrite_command(
                "SETOPTION name hash value 384"
            )
        finally:
            main.HASH = old_hash

        self.assertTrue(changed)
        self.assertEqual(command, "setoption name Hash value 1024")

    def test_unrelated_command_is_unchanged(self):
        original = "info string setoption name Hash value 1"
        command, changed = main.rewrite_command(original)
        self.assertFalse(changed)
        self.assertEqual(command, original)


if __name__ == "__main__":
    unittest.main()
