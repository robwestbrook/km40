-- Backfill check: counts observations, marked months, and month-end days (UTC) that have no reports.
-- Run it against a COPY of km40.db. Empty month-ends left after a full backfill are real IEM gaps.
SELECT 'total', COUNT(*) FROM observations;
SELECT 'marked_months', COUNT(*) FROM backfill;
SELECT 'flag', COUNT(*) FROM cache WHERE key='backfill_end_exclusive';
WITH RECURSIVE m(ym) AS (SELECT '2012-01-01' UNION ALL SELECT date(ym,'+1 month') FROM m WHERE ym < '2026-10-01'),
 e AS (SELECT date(ym,'+1 month','-1 day') AS day FROM m),
 d AS (SELECT date(obs_time,'unixepoch') AS day, COUNT(*) n FROM observations GROUP BY 1)
SELECT 'month_end_days_total', COUNT(*), 'with_zero_reports', SUM(d.day IS NULL) FROM e LEFT JOIN d ON d.day = e.day;
