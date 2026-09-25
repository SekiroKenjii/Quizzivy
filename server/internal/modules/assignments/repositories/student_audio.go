package repositories

const studentAudioSummary = `
 LEFT JOIN LATERAL (
   SELECT count(*) > 0 AS has_audio,
          coalesce(bool_or(recordings.shared),false) AS has_shared_audio,
          coalesce(bool_or(recordings.show_transcript),false) AS shows_transcript,
          min(recordings.max_plays) AS max_plays
   FROM (
     SELECT false AS shared, q.audio_show_transcript_after AS show_transcript,
            q.audio_max_plays AS max_plays
     FROM app.test_version_questions q
     JOIN app.test_version_sections s ON s.id=q.test_version_section_id
     WHERE s.test_version_id=a.test_version_id AND q.media_asset_kind='audio'
     UNION ALL
     SELECT true, r.show_transcript_after_submit, r.max_plays
     FROM app.test_version_group_recordings r
     JOIN app.test_version_groups g ON g.id=r.group_id
     JOIN app.test_version_sections s ON s.id=g.test_version_section_id
     WHERE s.test_version_id=a.test_version_id
   ) recordings
 ) audio ON true`
