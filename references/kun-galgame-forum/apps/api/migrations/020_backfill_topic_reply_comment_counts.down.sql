-- 020 (down): no-op. The backfilled values are the CORRECT counts; reverting to
-- the prior drifted/zero counts serves no purpose.
DO $$ BEGIN
  RAISE NOTICE '020 down is a no-op: corrected topic counts are intentionally kept';
END $$;
