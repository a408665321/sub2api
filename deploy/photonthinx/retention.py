"""Generate a Compose retention override from reviewed effective retention values.

Prints only the two retention settings. Does not connect, deploy, or read secrets.
"""
import argparse


def retention_floor(usage_logs_days: int, billing_dedup_days: int) -> tuple[int, int]:
    if usage_logs_days <= 0 or billing_dedup_days <= 0:
        raise ValueError('Review disabled/zero retention before enabling cleanup; supply positive effective values.')
    logs = max(400, usage_logs_days)
    return logs, max(400, billing_dedup_days, logs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--usage-logs-days', type=int, required=True)
    parser.add_argument('--usage-billing-dedup-days', type=int, required=True)
    args = parser.parse_args()
    try:
        logs, dedup = retention_floor(args.usage_logs_days, args.usage_billing_dedup_days)
    except ValueError as error:
        parser.error(str(error))
    print('services:\n  sub2api:\n    environment:')
    print(f'      DASHBOARD_AGGREGATION_RETENTION_USAGE_LOGS_DAYS: "{logs}"')
    print(f'      DASHBOARD_AGGREGATION_RETENTION_USAGE_BILLING_DEDUP_DAYS: "{dedup}"')


if __name__ == '__main__':
    main()
