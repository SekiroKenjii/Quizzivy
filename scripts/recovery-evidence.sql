BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT jsonb_build_object(
  'postgresMajor', current_setting('server_version_num')::int / 10000,
  'migration', (SELECT max(version_id) FROM public.goose_db_version WHERE is_applied),
  'unvalidatedConstraints', (SELECT count(oid) FROM pg_constraint WHERE connamespace='app'::regnamespace AND NOT convalidated),
  'appCanRewriteLogs', has_table_privilege('quizzivy_app','app.attempt_events','UPDATE,DELETE') OR has_table_privilege('quizzivy_app','app.audit_log','UPDATE,DELETE'),
  'users', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,email,full_name,role,disabled_at),0)::numeric),0)) FROM app.users),
  'versions', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,test_id,version,total_points,published_at),0)::numeric),0)) FROM app.test_versions),
  'questions', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,test_version_section_id,prompt,points,sample_answer,media_asset_id),0)::numeric),0)) FROM app.test_version_questions),
  'attempts', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,assignment_id,student_id,status,started_at,submitted_at,deadline_at),0)::numeric),0)) FROM app.attempts),
  'answers', (SELECT jsonb_build_object('rows',count(question_id),'digest',coalesce(sum(hashtextextended(concat_ws('|',attempt_id,question_id,payload,auto_score,manual_score),0)::numeric),0)) FROM app.attempt_answers),
  'events', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,attempt_id,session_id,kind,occurred_at,received_at,meta),0)::numeric),0)) FROM app.attempt_events),
  'audit', (SELECT jsonb_build_object('rows',count(id),'digest',coalesce(sum(hashtextextended(concat_ws('|',id,action,entity,entity_id,occurred_at,diff),0)::numeric),0)) FROM app.audit_log),
  'media', (SELECT coalesce(jsonb_agg(jsonb_build_object('id',id,'key',storage_key,'sha256',encode(checksum_sha256,'hex')) ORDER BY id),'[]'::jsonb) FROM app.media_assets WHERE deleted_at IS NULL OR id IN (SELECT media_asset_id FROM app.test_version_questions WHERE media_asset_id IS NOT NULL))
);
COMMIT;
