import unittest

from retention import retention_floor


class RetentionTests(unittest.TestCase):
    def test_raises_defaults_to_400(self):
        self.assertEqual(retention_floor(90, 365), (400, 400))

    def test_preserves_longer_values_and_dedup_invariant(self):
        self.assertEqual(retention_floor(730, 800), (730, 800))
        self.assertEqual(retention_floor(730, 400), (730, 730))
        self.assertEqual(retention_floor(90, 800), (400, 800))

    def test_ambiguous_disabled_retention_is_not_silently_enabled(self):
        for values in [(0, 365), (-1, 400), (400, 0)]:
            with self.assertRaises(ValueError):
                retention_floor(*values)


if __name__ == '__main__':
    unittest.main()
