package repositories

const draftQuestionRows = `
 SELECT q.id, q.points, q.media_asset_kind, q.context_group_id, q.tags
 FROM app.test_sections s
 JOIN app.test_section_questions sq ON sq.test_section_id=s.id
 JOIN app.questions q ON q.id=sq.question_id AND q.deleted_at IS NULL
 WHERE s.test_id=t.id
 UNION ALL
 SELECT q.id, q.points, q.media_asset_kind, q.context_group_id, q.tags
 FROM app.test_sections s
 JOIN app.question_groups g ON g.owner_section_id=s.id
 JOIN app.questions q ON q.context_group_id=g.id
 WHERE s.test_id=t.id`
