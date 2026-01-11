create table if not exists scheduler_runs (
  id bigserial primary key,
  job_name text not null,
  started_at timestamptz not null,
  finished_at timestamptz not null,
  success boolean not null,
  error text
);

create index if not exists idx_scheduler_runs_job_started on scheduler_runs (job_name, started_at desc);
