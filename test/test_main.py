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
    def write_example(self, directory, **overrides):
        cfg = copy.deepcopy(main.DEFAULTS)
        cfg.update(overrides)
        path = os.path.join(directory, main.CONFIG_EXAMPLE_FILENAME)
        with open(path, "w", encoding="utf-8") as file:
            json.dump(cfg, file)
        return cfg

    def test_missing_config_is_created_for_editing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            example = self.write_example(temp_dir, hash_mb=1024)
            with mock.patch.object(main, "script_dir", return_value=temp_dir):
                cfg, config_path, created = main.load_config()

            self.assertTrue(created)
            self.assertTrue(os.path.isfile(config_path))
            self.assertEqual(cfg["engine_path"], example["engine_path"])
            self.assertEqual(cfg["hash_mb"], 1024)

    def test_load_config_fills_new_defaults(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            self.write_example(temp_dir)
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

    def test_validate_config_rejects_log_path_outside_logs(self):
        cfg = copy.deepcopy(main.DEFAULTS)
        cfg["engine_path"] = os.path.abspath(__file__)
        cfg["log"]["file_path"] = os.path.join("..", "outside.log")
        with self.assertRaisesRegex(ValueError, "logs directory"):
            main.validate_config(cfg)

    def test_invalid_engine_path_is_requested_and_saved(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            engine_path = os.path.join(temp_dir, "pikafish.exe")
            Path(engine_path).touch()
            config_path = os.path.join(temp_dir, main.CONFIG_FILENAME)
            cfg = copy.deepcopy(main.DEFAULTS)

            forward_slash_path = engine_path.replace("\\", "/")
            main.ensure_engine_path(
                cfg,
                config_path,
                input_fn=lambda _: f'"{forward_slash_path}"',
            )

            self.assertEqual(cfg["engine_path"], os.path.normpath(engine_path))
            with open(config_path, "r", encoding="utf-8") as file:
                saved = json.load(file)
            self.assertEqual(saved["engine_path"], os.path.normpath(engine_path))

    def test_normalize_engine_path_accepts_backslashes(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            path_with_backslashes = temp_dir.replace("/", "\\") + "\\pikafish.exe"
            normalized = main.normalize_engine_path(path_with_backslashes)
            self.assertEqual(
                normalized,
                os.path.abspath(os.path.join(temp_dir, "pikafish.exe")),
            )


class LogTests(unittest.TestCase):
    def test_log_file_is_created_under_script_logs_directory(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            with (
                mock.patch.object(main, "script_dir", return_value=temp_dir),
                mock.patch.object(main, "LOG_FILE", True),
                mock.patch.object(main, "LOG_FILE_PATH", os.path.join("nested", "server.log")),
            ):
                handle = main._open_log_file()
                handle.write("test\n")
                handle.close()

            expected = os.path.join(temp_dir, "logs", "nested", "server.log")
            self.assertTrue(os.path.isfile(expected))


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
