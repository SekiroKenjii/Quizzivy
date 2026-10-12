-- Development seed, part eight: a teacher's whole workspace (R4, T-R4.51).
--
-- Follows 06 and 07 and relies on 06's teacher: giaovien@quizzivy.com (Trần
-- Thị Lan, …000000000601) owns everything here, so signing in as her opens
-- every teacher page with data shaped like the Teacher deck's fixtures: seven
-- tests, eight media files, four classes, assignments in every status, a
-- flagged attempt, answers waiting to be graded and four imports. Nothing here
-- reads 07's students; they stay in no class for T-R2.15.
--
-- The tests:
--   Mid-term Reading Mock       published, three versions, a draft ahead of the third
--   Unit 4 · Listening          published; a question of each content kind: a table and a
--                               link, two gaps, Markdown, audio, an image with its alt text
--   Vocabulary Quiz: Travel     published, two versions
--   Present Perfect vs Past     published
--   Writing Task 1 Practice     published, essays on an image
--   Speaking Part 2             draft
--   TOEIC Part 5                draft, committed from an import
--
-- Every row names its owner (owner_id, teacher_id, created_by). A published
-- version is written the way publish writes it: delivery_version group_v1,
-- the draft's sections, questions, options and blanks copied, the test moved
-- to published. Media rows are metadata only; no object is uploaded (see
-- README.md, "Media objects").
--
-- Ids are fixed. pg_temp.sid(kind, n) is …00000080 + kind + n and names what
-- a test or an assertion must find again; pg_temp.did(label) derives the rest
-- from a label. Kinds: 1 named students, 2 other students, 3 classes, 4 media,
-- 5 tests, 6 bank questions, 7 assignments, 8 imports (n 1 to 4; n + 16 is an
-- import's first source, + 32 its second, + 48 its run).
--
-- Idempotent like the other seeds: a row that exists is left as it is, and
-- a version is frozen only while its number is missing. The whole file is one
-- transaction.

BEGIN;

CREATE FUNCTION pg_temp.sid(kind int, n int) RETURNS uuid
LANGUAGE sql IMMUTABLE AS $$
  SELECT ('01935000-0000-7000-8000-00000080' || lpad(to_hex(kind), 2, '0') || lpad(to_hex(n), 2, '0'))::uuid
$$;

CREATE FUNCTION pg_temp.did(label text) RETURNS uuid
LANGUAGE sql IMMUTABLE AS $$
  SELECT ('01935000-0000-7000-8000-' || substr(md5('08-dev-teacher-workspace:' || label), 1, 12))::uuid
$$;

CREATE FUNCTION pg_temp.lan() RETURNS uuid
LANGUAGE sql IMMUTABLE AS $$ SELECT '01935000-0000-7000-8000-000000000601'::uuid $$;

CREATE FUNCTION pg_temp.doc(body text) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
  SELECT jsonb_build_object('format', 'semantic_v1', 'blocks', jsonb_build_array(
    jsonb_build_object('type', 'paragraph', 'content', jsonb_build_array(
      jsonb_build_object('type', 'text', 'text', body, 'marks', '[]'::jsonb)))))
$$;

-- ------------------------------------------------------------------ students
-- The ten named students are the deck's roster. Quốc Huy and Đức Minh hold a
-- temporary password, the deck's "Password reset" and "Not signed in".
INSERT INTO app.users (id, email, full_name, role_id, password_hash, must_change_password, created_by)
SELECT pg_temp.sid(1, v.n), v.email, v.full_name,
       (SELECT id FROM app.roles WHERE builtin_key = 'student'),
       '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
       v.temporary, pg_temp.lan()
  FROM (VALUES
    (1,  'minhanh.tran@quizzivy.com',     'Trần Minh Anh',    false),
    (2,  'nam.le@quizzivy.com',           'Lê Hoàng Nam',     false),
    (3,  'thuha.pham@quizzivy.com',       'Phạm Thu Hà',      false),
    (4,  'giabao.nguyen@quizzivy.com',    'Nguyễn Gia Bảo',   false),
    (5,  'khanhlinh.vo@quizzivy.com',     'Võ Khánh Linh',    false),
    (6,  'ducminh.hoang@quizzivy.com',    'Hoàng Đức Minh',   true),
    (7,  'quochuy.dang@quizzivy.com',     'Đặng Quốc Huy',    true),
    (8,  'phuongthao.do@quizzivy.com',    'Đỗ Phương Thảo',   false),
    (9,  'ngocmai.bui@quizzivy.com',      'Bùi Ngọc Mai',     false),
    (10, 'thanhtung.ngo@quizzivy.com',    'Ngô Thanh Tùng',   false)
  ) AS v(n, email, full_name, temporary)
ON CONFLICT DO NOTHING;

-- Fifty-nine more fill the classes to the deck's sizes: 1 to 14 IELTS 6.5
-- Evening, 15 to 28 IELTS Foundation A, 29 to 48 TOEIC 600 Weekend and 49 to
-- 59 Kids Starters B.
INSERT INTO app.users (id, email, full_name, role_id, password_hash, must_change_password, created_by)
SELECT pg_temp.sid(2, n),
       'hocvien.' || CASE WHEN n <= 14 THEN 'ielts65' WHEN n <= 28 THEN 'foundation'
                          WHEN n <= 48 THEN 'toeic600' ELSE 'kids' END || '.' || lpad(n::text, 2, '0') || '@quizzivy.com',
       (ARRAY['Nguyễn', 'Trần', 'Lê', 'Phạm', 'Hoàng', 'Huỳnh', 'Phan', 'Vũ', 'Võ', 'Đặng', 'Bùi', 'Đỗ'])[1 + (n * 7) % 12]
         || ' ' || (ARRAY['Văn', 'Thị', 'Minh', 'Ngọc', 'Thu', 'Quang', 'Hữu', 'Gia'])[1 + (n * 5) % 8]
         || ' ' || (ARRAY['An', 'Bình', 'Châu', 'Dương', 'Giang', 'Hải', 'Khôi', 'Lam', 'Mỹ', 'Nhi',
                          'Oanh', 'Phúc', 'Quyên', 'Sơn', 'Trâm', 'Uyên', 'Việt', 'Xuân', 'Yến', 'Đạt'])[1 + (n * 3) % 20],
       (SELECT id FROM app.roles WHERE builtin_key = 'student'),
       '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
       false, pg_temp.lan()
  FROM generate_series(1, 59) AS n
ON CONFLICT DO NOTHING;

-- ------------------------------------------------------------------- classes
INSERT INTO app.classes (id, name, description, self_join_enabled, teacher_id, schedule_label, room, created_at)
VALUES
  (pg_temp.sid(3, 1), 'IELTS 6.5 Evening', 'Lớp luyện thi IELTS buổi tối, mục tiêu 6.5.',
   true, pg_temp.lan(), 'Mon · Wed · Fri, 18:00', 'Room 3', now() - interval '120 days'),
  (pg_temp.sid(3, 2), 'IELTS Foundation A', 'Lớp nền tảng IELTS cho người mới bắt đầu.',
   true, pg_temp.lan(), 'Tue · Thu, 17:30', 'Room 2', now() - interval '90 days'),
  (pg_temp.sid(3, 3), 'TOEIC 600 Weekend', 'Lớp TOEIC cuối tuần, mục tiêu 600 điểm.',
   true, pg_temp.lan(), 'Sat · Sun, 09:00', 'Room 5', now() - interval '60 days'),
  (pg_temp.sid(3, 4), 'Kids Starters B', 'Lớp tiếng Anh thiếu nhi, trình độ Starters.',
   false, pg_temp.lan(), 'Sat, 14:00', 'Room 1', now() - interval '45 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.class_members (class_id, user_id, joined_via, joined_at, added_by)
SELECT pg_temp.sid(3, m.class_n), m.user_id, 'admin', now() - interval '30 days', pg_temp.lan()
  FROM (
    SELECT 1 AS class_n, pg_temp.sid(1, n) AS user_id FROM generate_series(1, 10) AS n
    UNION ALL SELECT 1, pg_temp.sid(2, n) FROM generate_series(1, 14) AS n
    UNION ALL SELECT 2, pg_temp.sid(1, n) FROM unnest(ARRAY[3, 4]) AS n
    UNION ALL SELECT 2, pg_temp.sid(2, n) FROM generate_series(15, 28) AS n
    UNION ALL SELECT 3, pg_temp.sid(1, n) FROM unnest(ARRAY[5, 8]) AS n
    UNION ALL SELECT 3, pg_temp.sid(2, n) FROM generate_series(29, 48) AS n
    UNION ALL SELECT 4, pg_temp.sid(1, 6)
    UNION ALL SELECT 4, pg_temp.sid(2, n) FROM generate_series(49, 59) AS n
  ) AS m
ON CONFLICT (class_id, user_id) DO NOTHING;

-- --------------------------------------------------------------------- media
-- Metadata only. The checksum is the SHA-256 of the storage key, not of any
-- bytes, because no object exists. The podcast is 4:58 where the deck says
-- 6:05: media_assets_duration_ms_check refuses an audio file over five minutes.
INSERT INTO app.media_assets
  (id, kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256,
   uploaded_by, created_at, owner_id, display_name, default_max_plays, width, height)
SELECT pg_temp.sid(4, v.n), v.kind::app.media_kind,
       v.kind || '/' || pg_temp.sid(4, v.n) || v.ext, v.mime, v.bytes, v.duration_ms, v.name,
       sha256(convert_to(v.kind || '/' || pg_temp.sid(4, v.n) || v.ext, 'UTF8')),
       pg_temp.lan(), now() - v.age, pg_temp.lan(), v.name, v.plays, v.width, v.height
  FROM (VALUES
    (1, 'audio', '.mp3', 'audio/mpeg', 3984588,  252000, 'Cambridge 15 · Test 2 · Part 1.mp3',       2,    NULL, NULL, interval '40 days'),
    (2, 'image', '.png', 'image/png',   245760,  NULL,   'Directions map.png',                      NULL, 1200, 800,  interval '30 days'),
    (3, 'audio', '.mp3', 'audio/mpeg', 2516582,  156000, 'Unit 4 · Airport announcements.mp3',      1,    NULL, NULL, interval '28 days'),
    (4, 'image', '.png', 'image/png',   184320,  NULL,   'Chart · internet access 2000–2020.png',   NULL, 1600, 900,  interval '21 days'),
    (5, 'audio', '.mp3', 'audio/mpeg', 5872026,  298000, 'Podcast · city parks.mp3',                0,    NULL, NULL, interval '14 days'),
    (6, 'image', '.jpg', 'image/jpeg',  317440,  NULL,   'Floor plan · library.jpg',                NULL, 1024, 768,  interval '10 days'),
    (7, 'audio', '.mp3', 'audio/mpeg',  524288,  48000,  'Minimal pairs · ship, sheep.mp3',        3,    NULL, NULL, interval '6 days'),
    (8, 'audio', '.mp3', 'audio/mpeg', 1153434,  80000,  'Kids · animal sounds.mp3',                0,    NULL, NULL, interval '3 days')
  ) AS v(n, kind, ext, mime, bytes, duration_ms, name, plays, width, height, age)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------- the bank
-- put_question writes one bank question with its options and blanks from a
-- jsonb spec: type, prompt, points, level, skill, tags, options, correct (1-based
-- positions), blanks (answers, gap, caseSensitive), sample, explanation, content
-- (the prose document of the prompt), rich (prompt and options as prose),
-- media (media n), alt, audio (maxPlays, allowSeek, showTranscript, allowPause), transcript
-- and ageHours. A question that exists is left as the teacher has it.
CREATE FUNCTION pg_temp.put_question(n int, spec jsonb) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
  qid uuid := pg_temp.sid(6, n);
  media_id uuid := CASE WHEN spec ? 'media' THEN pg_temp.sid(4, (spec->>'media')::int) END;
  rich boolean := coalesce((spec->>'rich')::boolean, false);
  opts text[] := ARRAY(SELECT jsonb_array_elements_text(coalesce(spec->'options', '[]'::jsonb)));
  correct int[] := ARRAY(SELECT jsonb_array_elements_text(coalesce(spec->'correct', '[]'::jsonb))::int);
  age interval := make_interval(hours => coalesce((spec->>'ageHours')::int, 240));
  blank jsonb;
  blank_id uuid;
  position int := 0;
  answer text;
  inserted int;
BEGIN
  INSERT INTO app.questions
    (id, type, prompt, media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
     audio_show_transcript_after, transcript, points, explanation, sample_answer, tags,
     created_by, owner_id, created_at, updated_at, prompt_content, level, skill, media_alt, audio_allow_pause)
  VALUES (
    qid, (spec->>'type')::app.question_type, spec->>'prompt', media_id,
    (SELECT kind FROM app.media_assets WHERE id = media_id),
    (spec#>>'{audio,maxPlays}')::int,
    CASE WHEN spec ? 'audio' THEN coalesce((spec#>>'{audio,allowSeek}')::boolean, false) END,
    CASE WHEN spec ? 'audio' THEN coalesce((spec#>>'{audio,showTranscript}')::boolean, false) END,
    spec->>'transcript', (spec->>'points')::numeric, spec->>'explanation', spec->>'sample',
    ARRAY(SELECT jsonb_array_elements_text(coalesce(spec->'tags', '[]'::jsonb))),
    pg_temp.lan(), pg_temp.lan(), now() - age, now() - age,
    CASE WHEN rich THEN pg_temp.doc(spec->>'prompt') ELSE spec->'content' END,
    spec->>'level', spec->>'skill', spec->>'alt',
    CASE WHEN spec ? 'audio' THEN coalesce((spec#>>'{audio,allowPause}')::boolean, true) END)
  ON CONFLICT (id) DO NOTHING;
  GET DIAGNOSTICS inserted = ROW_COUNT;
  IF inserted = 0 THEN
    RETURN;
  END IF;

  FOR i IN 1..coalesce(array_length(opts, 1), 0) LOOP
    INSERT INTO app.question_options (id, question_id, ordinal, text, is_correct, content)
    VALUES (pg_temp.did('qo:' || qid || ':' || i), qid, i - 1, opts[i], i = ANY (correct),
            CASE WHEN rich THEN pg_temp.doc(opts[i]) END);
  END LOOP;

  FOR blank IN SELECT * FROM jsonb_array_elements(coalesce(spec->'blanks', '[]'::jsonb)) LOOP
    position := position + 1;
    blank_id := pg_temp.did('qb:' || qid || ':' || position);
    INSERT INTO app.question_blanks (id, question_id, ordinal, case_sensitive, gap_id)
    VALUES (blank_id, qid, position, coalesce((blank->>'caseSensitive')::boolean, false), blank->>'gap');
    FOR answer IN SELECT jsonb_array_elements_text(blank->'answers') LOOP
      INSERT INTO app.question_blank_answers (id, blank_id, answer)
      VALUES (pg_temp.did('qba:' || blank_id || ':' || answer), blank_id, answer);
    END LOOP;
  END LOOP;
END;
$$;

-- Mid-term Reading Mock (1 to 9). 4 is edited after the third version; 9 is
-- in the draft only.
SELECT pg_temp.put_question(1, $q$ {"type":"single_choice","prompt":"What is the main purpose of the passage?","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":120,
  "options":["To argue that cities should invest in public green space","To describe the history of city planning","To compare parks in three countries","To explain how to design a garden"],"correct":[1]} $q$);
SELECT pg_temp.put_question(2, $q$ {"type":"single_choice","prompt":"What does the writer say about parks in paragraph B?","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":120,
  "options":["They are mainly used by older residents.","They improve both health and property values.","They cost more to maintain than they return.","They should be replaced by indoor facilities."],"correct":[2]} $q$);
SELECT pg_temp.put_question(3, $q$ {"type":"single_choice","prompt":"True, False or Not Given: The author supports car-free zones.","points":1,"level":"b2","skill":"reading","tags":["reading","TFNG"],"ageHours":96,
  "options":["True","False","Not Given"],"correct":[1]} $q$);
SELECT pg_temp.put_question(4, $q$ {"type":"short_answer","prompt":"According to paragraph C, what should cities prioritise when planning new districts? Write NO MORE THAN TWO WORDS.","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":120,
  "sample":"public green space"} $q$);
SELECT pg_temp.put_question(5, $q$ {"type":"single_choice","prompt":"Why were early maps often inaccurate?","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":120,
  "options":["Mapmakers relied on reports from travellers","Mapmakers refused to leave their cities","Paper was too expensive to use","Ink faded within a few years"],"correct":[1]} $q$);
SELECT pg_temp.put_question(6, $q$ {"type":"fill_blank","prompt":"Complete the sentence with ONE word from the passage: early mapmakers relied on {{1}} from travellers.","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":72,
  "blanks":[{"answers":["reports"]}]} $q$);
SELECT pg_temp.put_question(7, $q$ {"type":"multiple_choice","prompt":"Which statements match the writer’s view?","points":2,"level":"b2","skill":"reading","tags":["reading"],"ageHours":72,
  "options":["Maps have always been drawn from direct observation.","Maps reflect what their makers believed about the world.","Accurate maps needed better instruments as well as better travellers.","Travellers’ accounts were always reliable."],"correct":[2,3]} $q$);
SELECT pg_temp.put_question(8, $q$ {"type":"short_answer","prompt":"What term does the writer use for streets closed to cars at weekends?","points":1,"level":"b1","skill":"reading","tags":["reading"],"ageHours":72,
  "sample":"play streets"} $q$);
SELECT pg_temp.put_question(9, $q$ {"type":"single_choice","prompt":"What is the writer’s attitude to indoor sports facilities?","points":1,"level":"b2","skill":"reading","tags":["reading"],"ageHours":48,
  "options":["Enthusiastic","Critical","Neutral","Uncertain"],"correct":[3]} $q$);

-- Vocabulary Quiz: Travel (10 to 15).
SELECT pg_temp.put_question(10, $q$ {"type":"multiple_choice","prompt":"Which of the following are means of transport? Select all.","points":2,"level":"a1","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":336,
  "options":["Bus","Ferry","Suitcase","Metro","Passport"],"correct":[1,2,4]} $q$);
SELECT pg_temp.put_question(11, $q$ {"type":"fill_blank","prompt":"The {{1}} board shows which gate your flight leaves from.","points":1,"level":"a2","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":336,
  "blanks":[{"answers":["departure"]}]} $q$);
SELECT pg_temp.put_question(12, $q$ {"type":"single_choice","prompt":"A ticket that covers the journey there and back is called a ___ ticket.","points":1,"level":"a2","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":336,
  "options":["single","return","open","direct"],"correct":[2]} $q$);
SELECT pg_temp.put_question(13, $q$ {"type":"single_choice","prompt":"Which word means a plan of a journey, day by day?","points":1,"level":"a2","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":336,
  "options":["itinerary","commute","fare","delay"],"correct":[1]} $q$);
SELECT pg_temp.put_question(14, $q$ {"type":"fill_blank","prompt":"There is a 20-minute {{1}} because of the fog.","points":1,"level":"a2","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":300,
  "blanks":[{"answers":["delay"]}]} $q$);
SELECT pg_temp.put_question(15, $q$ {"type":"fill_blank","prompt":"The bus {{1}} went up this year.","points":1,"level":"a2","skill":"vocabulary","tags":["vocabulary","travel"],"ageHours":300,
  "blanks":[{"answers":["fare"]}]} $q$);

-- Present Perfect vs Past Simple (20 to 24).
SELECT pg_temp.put_question(20, $q$ {"type":"single_choice","prompt":"Choose the correct form: She ___ in Hanoi since 2019.","points":1,"level":"a2","skill":"grammar","tags":["grammar","present perfect"],"ageHours":48,
  "options":["lives","has lived","lived","is living"],"correct":[2]} $q$);
SELECT pg_temp.put_question(21, $q$ {"type":"fill_blank","prompt":"Write the missing word: \"I look forward {{1}} hearing from you.\"","points":1,"level":"b1","skill":"grammar","tags":["grammar","prepositions"],"ageHours":168,
  "blanks":[{"answers":["to"]}]} $q$);
SELECT pg_temp.put_question(22, $q$ {"type":"single_choice","prompt":"I ___ him yesterday at the station.","points":1,"level":"a2","skill":"grammar","tags":["grammar","past simple"],"ageHours":48,
  "options":["saw","have seen","see","was seeing"],"correct":[1]} $q$);
SELECT pg_temp.put_question(23, $q$ {"type":"single_choice","prompt":"They ___ the report yet.","points":1,"level":"a2","skill":"grammar","tags":["grammar","present perfect"],"ageHours":48,
  "options":["didn’t finish","haven’t finished","don’t finish","hadn’t finished"],"correct":[2]} $q$);
SELECT pg_temp.put_question(24, $q$ {"type":"fill_blank","prompt":"We have lived here {{1}} 2015.","points":1,"level":"a2","skill":"grammar","tags":["grammar","present perfect"],"ageHours":48,
  "blanks":[{"answers":["since"]}]} $q$);

-- Unit 4 · Listening: Directions (30 to 34), one question of each content kind.
SELECT pg_temp.put_question(30, $q$ {"type":"single_choice","prompt":"Listen to the announcement. Which gate is the flight to Da Nang boarding from?","points":1,"level":"a2","skill":"listening","tags":["listening","audio"],"ageHours":168,
  "media":3,"audio":{"maxPlays":1,"allowSeek":false,"showTranscript":true,"allowPause":false},
  "transcript":"Attention, please. Flight VN 160 to Da Nang is now boarding at gate 12. Passengers should have their boarding passes ready.",
  "options":["Gate 8","Gate 12","Gate 21","Gate 2"],"correct":[2]} $q$);
SELECT pg_temp.put_question(31, $q$ {"type":"single_choice","prompt":"Look at the map and label it: where is the post office?","points":1,"level":"b1","skill":"listening","tags":["listening","map"],"ageHours":168,
  "media":2,"alt":"Street map of a small town centre. The post office stands on the corner of Bridge Street and Park Road, opposite the railway station.",
  "options":["Opposite the station","Next to the library","Behind the bank","At the end of Park Road"],"correct":[1]} $q$);
SELECT pg_temp.put_question(32, $q$ {"type":"short_answer","prompt":"**Listen again** and write down the *two* landmarks the speaker mentions.\n\n- the first is near the station\n- the second is a green space","points":2,"level":"b1","skill":"listening","tags":["listening"],"ageHours":168,
  "sample":"the bank; the park"} $q$);
SELECT pg_temp.put_question(33, $q$ {"type":"single_choice","points":1,"level":"b1","skill":"listening","tags":["listening","timetable"],"ageHours":168,
  "prompt":"Read the bus timetable, then answer the question.\n\nRoute\tDeparts\tArrives\nRoute 12\t08:10\t08:45\nRoute 27\t08:30\t09:20\n\nFull timetable: city transport website\n\nWhich bus arrives in the city centre first?",
  "content":{"format":"semantic_v1","blocks":[
    {"type":"paragraph","content":[{"type":"text","text":"Read the bus timetable, then answer the question.","marks":[]}]},
    {"type":"table","rows":[
      [{"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Route","marks":["bold"]}]}]},
       {"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Departs","marks":["bold"]}]}]},
       {"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Arrives","marks":["bold"]}]}]}],
      [{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Route 12","marks":[]}]}]},
       {"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"08:10","marks":[]}]}]},
       {"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"08:45","marks":[]}]}]}],
      [{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Route 27","marks":[]}]}]},
       {"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"08:30","marks":[]}]}]},
       {"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"09:20","marks":[]}]}]}]]},
    {"type":"paragraph","content":[{"type":"text","text":"Full timetable: ","marks":[]},
      {"type":"link","href":"https://www.example.org/bus-timetable","content":[{"type":"text","text":"city transport website","marks":["underline"]}]}]},
    {"type":"paragraph","content":[{"type":"text","text":"Which bus arrives in the city centre first?","marks":["bold"]}]}]},
  "options":["Route 12","Route 27"],"correct":[1]} $q$);
SELECT pg_temp.put_question(34, $q$ {"type":"fill_blank","points":2,"level":"a2","skill":"listening","tags":["listening","directions"],"ageHours":168,
  "prompt":"Turn [1] at the bank and walk [2] the park.",
  "content":{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[
    {"type":"text","text":"Turn ","marks":[]},{"type":"gap","id":"g1","label":"1"},
    {"type":"text","text":" at the bank and walk ","marks":[]},{"type":"gap","id":"g2","label":"2"},
    {"type":"text","text":" the park.","marks":[]}]}]},
  "blanks":[{"gap":"g1","answers":["left"]},{"gap":"g2","answers":["past","through","along"]}]} $q$);

-- Speaking Part 2 · Cue cards (40 to 42).
SELECT pg_temp.put_question(40, $q$ {"type":"short_answer","prompt":"Describe a place you visited recently. You should say where it was, who you went with and why you remember it.","points":3,"level":"b1","skill":"speaking","tags":["speaking","part 2"],"ageHours":5} $q$);
SELECT pg_temp.put_question(41, $q$ {"type":"short_answer","prompt":"Describe a person who has influenced you. You should say who the person is, how you know them and what you learned from them.","points":3,"level":"b1","skill":"speaking","tags":["speaking","part 2"],"ageHours":6} $q$);
SELECT pg_temp.put_question(42, $q$ {"type":"short_answer","prompt":"Describe a meal you remember well. You should say what you ate, where you ate it and why it was special.","points":3,"level":"b1","skill":"speaking","tags":["speaking","part 2"],"ageHours":7} $q$);

-- Writing Task 1 Practice (50, 51) and a bank question no test uses (52).
SELECT pg_temp.put_question(50, $q$ {"type":"short_answer","prompt":"The chart below shows the percentage of households with internet access in three countries between 2000 and 2020. Summarise the information by selecting and reporting the main features.","points":9,"level":"b2","skill":"writing","tags":["writing","task 1"],"ageHours":504,
  "media":4,"alt":"Bar chart of the percentage of households with internet access in three countries in 2000, 2010 and 2020. All three rise steeply, from under 20 per cent in 2000 to over 80 per cent in 2020.",
  "sample":"The bar chart illustrates the proportion of households with internet access in three countries between 2000 and 2020. Overall, access rose sharply in every country."} $q$);
SELECT pg_temp.put_question(51, $q$ {"type":"short_answer","prompt":"The plan shows the ground floor of a library before and after refurbishment. Summarise the information by selecting and reporting the main features.","points":9,"level":"b2","skill":"writing","tags":["writing","task 1"],"ageHours":504,
  "media":6,"alt":"Two floor plans of a library ground floor. Before: a single reading room with the desk by the entrance. After: the desk moves to the centre and a new computer area replaces two walls."} $q$);
SELECT pg_temp.put_question(52, $q$ {"type":"short_answer","prompt":"Summarise the chart in at least 150 words.","points":9,"level":"b2","skill":"writing","tags":["writing","task 1"],"ageHours":504} $q$);

-- TOEIC Part 5 · Incomplete sentences (60 to 65), as an import commits them:
-- prompt and options in prose.
SELECT pg_temp.put_question(60, $q$ {"type":"single_choice","rich":true,"prompt":"The manager asked all employees ___ the new safety rules.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["to follow","follow","following","followed"],"correct":[1]} $q$);
SELECT pg_temp.put_question(61, $q$ {"type":"single_choice","rich":true,"prompt":"Please send the invoice ___ Friday.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["by","at","on","in"],"correct":[1]} $q$);
SELECT pg_temp.put_question(62, $q$ {"type":"single_choice","rich":true,"prompt":"Ms. Tran is ___ for coordinating the training programme.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["responsible","respond","responsibly","response"],"correct":[1]} $q$);
SELECT pg_temp.put_question(63, $q$ {"type":"single_choice","rich":true,"prompt":"The conference was postponed ___ the heavy rain.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["because of","because","although","despite of"],"correct":[1]} $q$);
SELECT pg_temp.put_question(64, $q$ {"type":"single_choice","rich":true,"prompt":"All visitors must ___ at the front desk.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["register","registered","registering","registration"],"correct":[1]} $q$);
SELECT pg_temp.put_question(65, $q$ {"type":"single_choice","rich":true,"prompt":"The new software is ___ more efficient than the old one.","points":1,"level":"b1","skill":"grammar","tags":["grammar","TOEIC"],"ageHours":96,
  "options":["significantly","significant","signify","significance"],"correct":[1]} $q$);

-- ---------------------------------------------------------------- the tests
-- A published test is inserted with its final status and version number; the
-- version rows follow from freeze. updated_at is "edited … ago" in the list.
INSERT INTO app.tests (id, title, description, status, current_version, created_by, owner_id, created_at, updated_at)
SELECT pg_temp.sid(5, v.n), v.title, v.description, v.status::app.test_status, v.current_version,
       pg_temp.lan(), pg_temp.lan(), now() - v.created, now() - v.edited
  FROM (VALUES
    (1, 'Mid-term Reading Mock',                'Đề đọc hiểu giữa kỳ cho lớp IELTS 6.5, hai bài đọc.',                  'published', 3, interval '60 days', interval '3 days'),
    (2, 'Unit 4 · Listening: Directions',       'Bài nghe chỉ đường: có âm thanh, bản đồ, bảng giờ xe buýt và chỗ trống.', 'published', 1, interval '30 days', interval '7 days'),
    (3, 'Vocabulary Quiz: Travel',              'Từ vựng chủ đề du lịch và giao thông.',                                 'published', 2, interval '45 days', interval '14 days'),
    (4, 'Present Perfect vs Past Simple',       'Phân biệt thì hiện tại hoàn thành và quá khứ đơn.',                     'published', 1, interval '10 days', interval '1 day'),
    (5, 'Speaking Part 2 · Cue cards',          'Ba thẻ gợi ý cho phần nói Part 2.',                                     'draft',     0, interval '2 days',  interval '2 hours'),
    (6, 'Writing Task 1 Practice',              'Hai đề Writing Task 1: biểu đồ và sơ đồ mặt bằng.',                     'published', 1, interval '50 days', interval '21 days'),
    (7, 'TOEIC Part 5 · Incomplete sentences',  'Câu hỏi điền từ TOEIC Part 5, nhập từ tệp Word.',                       'draft',     0, interval '5 days',  interval '4 days')
  ) AS v(n, title, description, status, current_version, created, edited)
ON CONFLICT (id) DO NOTHING;

-- put_section writes a section if its id and its place are free and appends
-- the questions to it from start; a question already in the section, or an
-- ordinal already taken, is skipped.
CREATE FUNCTION pg_temp.put_section(test_n int, place int, heading text, intro text, first int, members int[])
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  section_id uuid := pg_temp.did('section:' || test_n || ':' || place);
BEGIN
  INSERT INTO app.test_sections (id, test_id, ordinal, title, instructions)
  SELECT section_id, pg_temp.sid(5, test_n), place, heading, intro
   WHERE NOT EXISTS (SELECT 1 FROM app.test_sections s
                      WHERE s.id = section_id OR (s.test_id = pg_temp.sid(5, test_n) AND s.ordinal = place));
  FOR i IN 1..coalesce(array_length(members, 1), 0) LOOP
    INSERT INTO app.test_section_questions (id, test_section_id, ordinal, question_id)
    SELECT pg_temp.did('member:' || section_id || ':' || members[i]), section_id, first + i - 1, pg_temp.sid(6, members[i])
     WHERE EXISTS (SELECT 1 FROM app.test_sections s WHERE s.id = section_id)
       AND NOT EXISTS (SELECT 1 FROM app.test_section_questions m
                        WHERE m.test_section_id = section_id
                          AND (m.question_id = pg_temp.sid(6, members[i]) OR m.ordinal = first + i - 1))
       AND NOT EXISTS (SELECT 1 FROM app.test_section_units u WHERE u.test_section_id = section_id);
  END LOOP;
END;
$$;

-- freeze snapshots a test's draft as version v, as publish does, unless that
-- version exists: the version, its sections, and each question with its
-- options, blanks and accepted answers.
CREATE FUNCTION pg_temp.freeze(test_n int, v int, note text, days_ago int) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
  tid uuid := pg_temp.sid(5, test_n);
  vid uuid := pg_temp.did('version:' || test_n || ':' || v);
BEGIN
  IF EXISTS (SELECT 1 FROM app.test_versions WHERE test_id = tid AND version = v) THEN
    RETURN;
  END IF;

  INSERT INTO app.test_versions (id, test_id, version, total_points, published_at, published_by, delivery_version, change_note)
  SELECT vid, tid, v, sum(q.points), now() - days_ago * interval '1 day', pg_temp.lan(), 'group_v1', note
    FROM app.test_sections s
    JOIN app.test_section_questions m ON m.test_section_id = s.id
    JOIN app.questions q ON q.id = m.question_id
   WHERE s.test_id = tid;

  INSERT INTO app.test_version_sections (id, test_version_id, ordinal, title, instructions)
  SELECT pg_temp.did('vsection:' || vid || ':' || s.ordinal), vid, s.ordinal, s.title, s.instructions
    FROM app.test_sections s WHERE s.test_id = tid;

  INSERT INTO app.test_version_questions
    (id, test_version_section_id, ordinal, source_question_id, type, prompt, media_asset_id, media_asset_kind,
     audio_max_plays, audio_allow_seek, audio_show_transcript_after, transcript, points, explanation, sample_answer,
     prompt_content, explanation_content, level, skill, media_alt, audio_allow_pause)
  SELECT pg_temp.did('vq:' || vid || ':' || q.id), pg_temp.did('vsection:' || vid || ':' || s.ordinal), m.ordinal, q.id,
         q.type, q.prompt, q.media_asset_id, q.media_asset_kind, q.audio_max_plays, q.audio_allow_seek,
         q.audio_show_transcript_after, q.transcript, q.points, q.explanation, q.sample_answer,
         q.prompt_content, q.explanation_content, q.level, q.skill, q.media_alt, q.audio_allow_pause
    FROM app.test_sections s
    JOIN app.test_section_questions m ON m.test_section_id = s.id
    JOIN app.questions q ON q.id = m.question_id
   WHERE s.test_id = tid;

  INSERT INTO app.test_version_options (id, test_version_question_id, ordinal, text, is_correct, content)
  SELECT pg_temp.did('vo:' || f.id || ':' || o.ordinal), f.id, o.ordinal, o.text, o.is_correct, o.content
    FROM app.test_version_questions f
    JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id AND vs.test_version_id = vid
    JOIN app.question_options o ON o.question_id = f.source_question_id;

  INSERT INTO app.test_version_blanks (id, test_version_question_id, ordinal, case_sensitive, gap_id)
  SELECT pg_temp.did('vb:' || f.id || ':' || b.ordinal), f.id, b.ordinal, b.case_sensitive, b.gap_id
    FROM app.test_version_questions f
    JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id AND vs.test_version_id = vid
    JOIN app.question_blanks b ON b.question_id = f.source_question_id;

  INSERT INTO app.test_version_blank_answers (id, test_version_blank_id, answer)
  SELECT pg_temp.did('vba:' || fb.id || ':' || a.answer), fb.id, a.answer
    FROM app.test_version_blanks fb
    JOIN app.test_version_questions f ON f.id = fb.test_version_question_id
    JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id AND vs.test_version_id = vid
    JOIN app.question_blanks b ON b.question_id = f.source_question_id AND b.ordinal = fb.ordinal
    JOIN app.question_blank_answers a ON a.blank_id = b.id;
END;
$$;

-- Mid-term Reading Mock: three versions, then a draft ahead of the third --
-- question 4's wording is corrected and question 9 is added.
SELECT pg_temp.put_section(1, 0, 'Passage 1 · Urban green space', 'Read the passage about urban green space and answer questions 1–4.', 0, ARRAY[1, 2, 3, 4]);
SELECT pg_temp.put_section(1, 1, 'Passage 2 · The history of maps', NULL, 0, ARRAY[5]);
SELECT pg_temp.freeze(1, 1, 'Bản đầu tiên: năm câu hỏi.', 50);
SELECT pg_temp.put_section(1, 1, 'Passage 2 · The history of maps', NULL, 1, ARRAY[6]);
SELECT pg_temp.freeze(1, 2, 'Thêm câu điền từ cho bài đọc thứ hai.', 30);
SELECT pg_temp.put_section(1, 1, 'Passage 2 · The history of maps', NULL, 2, ARRAY[7, 8]);
SELECT pg_temp.freeze(1, 3, 'Thêm câu nhiều lựa chọn và câu trả lời ngắn về play streets.', 10);
SELECT pg_temp.put_section(1, 1, 'Passage 2 · The history of maps', NULL, 4, ARRAY[9]);

UPDATE app.questions
   SET prompt = 'According to paragraph C, what should cities prioritise when planning new districts? Write NO MORE THAN THREE WORDS.'
 WHERE id = pg_temp.sid(6, 4)
   AND prompt LIKE '%TWO WORDS.';

-- Unit 4 · Listening: Directions.
SELECT pg_temp.put_section(2, 0, 'Phần 1 · Nghe và nhìn bản đồ', 'Nghe thông báo, xem bản đồ rồi trả lời.', 0, ARRAY[30, 31, 32]);
SELECT pg_temp.put_section(2, 1, 'Phần 2 · Đọc bảng giờ và chỉ đường', NULL, 0, ARRAY[33, 34]);
SELECT pg_temp.freeze(2, 1, 'Bản đầu tiên.', 20);

-- Vocabulary Quiz: Travel.
SELECT pg_temp.put_section(3, 0, 'Từ vựng du lịch', NULL, 0, ARRAY[10, 11, 12, 13]);
SELECT pg_temp.freeze(3, 1, 'Bản đầu tiên: bốn câu hỏi.', 40);
SELECT pg_temp.put_section(3, 0, 'Từ vựng du lịch', NULL, 4, ARRAY[14, 15]);
SELECT pg_temp.freeze(3, 2, 'Thêm hai câu điền từ.', 14);

-- Present Perfect vs Past Simple.
SELECT pg_temp.put_section(4, 0, 'Chọn hoặc điền từ', NULL, 0, ARRAY[20, 21, 22, 23, 24]);
SELECT pg_temp.freeze(4, 1, 'Bản đầu tiên.', 1);

-- Speaking Part 2 · Cue cards, a draft.
SELECT pg_temp.put_section(5, 0, 'Cue cards', NULL, 0, ARRAY[40, 41, 42]);

-- Writing Task 1 Practice.
SELECT pg_temp.put_section(6, 0, 'Task 1', 'Write at least 150 words in 20 minutes.', 0, ARRAY[50, 51]);
SELECT pg_temp.freeze(6, 1, 'Bản đầu tiên.', 30);

-- TOEIC Part 5 · Incomplete sentences, committed from an import.
SELECT pg_temp.put_section(7, 0, 'Part 5 · Incomplete sentences', NULL, 0, ARRAY[60, 61, 62, 63, 64, 65]);

-- ------------------------------------------------------------- assignments
-- One in every status. The windows are long so a database seeded today still
-- shows them open next week; 4 opens in three days, 5 closed nine days ago and
-- 6 is a draft: it has no published_at, so no student sees it.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts,
   shuffle_questions, shuffle_options, review_show_score, review_show_correct_answers, review_show_explanations,
   integrity_require_fullscreen, integrity_max_focus_loss, integrity_on_limit_exceeded,
   created_by, created_at, published_at, review_release, review_show_class_average, student_note)
SELECT pg_temp.sid(7, v.n), pg_temp.sid(5, v.test_n), pg_temp.did('version:' || v.test_n || ':' || v.version),
       now() + v.opens, now() + v.closes, v.duration, v.max_attempts,
       v.shuffle, v.shuffle, true, v.show_answers, v.show_answers,
       v.fullscreen, v.max_focus_loss, 'flag',
       pg_temp.lan(), now() + v.created, CASE WHEN v.published THEN now() + v.created END,
       'on_submit', v.show_answers, v.note
  FROM (VALUES
    (1, 1, 3, interval '-2 days',  interval '3 days',   60, 1, true,  false, true,  2,  true,  interval '-3 days',  'Nên dùng máy tính xách tay nếu có thể; đoạn văn dễ đọc hơn trên màn hình lớn.'),
    (2, 2, 1, interval '-3 days',  interval '11 days',  30, 2, false, false, false, 0,  true,  interval '-4 days',  NULL),
    (3, 3, 2, interval '-5 days',  interval '9 days',   20, 1, true,  true,  false, 0,  true,  interval '-6 days',  NULL),
    (4, 4, 1, interval '3 days',   interval '5 days',   40, 1, false, false, false, 0,  true,  interval '-1 day',   'Bài mở vào thứ Hai; chuẩn bị tai nghe không cần thiết.'),
    (5, 6, 1, interval '-14 days', interval '-9 days',  45, 1, false, false, true,  1,  true,  interval '-15 days', NULL),
    (6, 3, 1, interval '7 days',   interval '9 days',   15, 1, false, false, false, 0,  false, interval '-2 hours', NULL)
  ) AS v(n, test_n, version, opens, closes, duration, max_attempts, shuffle, fullscreen, show_answers, max_focus_loss, published, created, note)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
VALUES
  (pg_temp.sid(7, 1), pg_temp.sid(3, 1)),
  (pg_temp.sid(7, 2), pg_temp.sid(3, 2)),
  (pg_temp.sid(7, 3), pg_temp.sid(3, 3)),
  (pg_temp.sid(7, 3), pg_temp.sid(3, 4)),
  (pg_temp.sid(7, 4), pg_temp.sid(3, 2)),
  (pg_temp.sid(7, 5), pg_temp.sid(3, 1)),
  (pg_temp.sid(7, 6), pg_temp.sid(3, 3))
ON CONFLICT DO NOTHING;

INSERT INTO app.assignment_student_overrides (assignment_id, student_id, extra_attempts, reason, created_by)
VALUES (pg_temp.sid(7, 1), pg_temp.sid(1, 7), 1, 'Mất kết nối mạng giữa chừng, cho làm lại một lần.', pg_temp.lan())
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------- attempts
-- put_attempt writes one student's attempt with an answer to each of the first
-- `answered` questions of the assignment's version, the first `right` of them
-- correct. outcome is graded (every manual answer scored), pending (submitted,
-- manual answers waiting), timed_out (pending, at the deadline) or in_progress.
-- Scores are summed from the answers as the product does. A flagged attempt
-- gets the integrity events that flagged it.
CREATE FUNCTION pg_temp.put_attempt(assignment_n int, student uuid, outcome text, answered int, right_count int,
                                    spent_minutes int, started_ago interval, focus_losses int, flagged boolean)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  aid uuid := pg_temp.did('attempt:' || assignment_n || ':' || student);
  session uuid := pg_temp.did('session:' || assignment_n || ':' || student);
  asg record;
  started timestamptz;
  submitted timestamptz;
  at_status app.attempt_status;
  q record;
  position int := 0;
  is_right boolean;
  manual boolean;
  payload jsonb;
  graded_manually boolean := outcome = 'graded';
  earned numeric;
BEGIN
  IF EXISTS (SELECT 1 FROM app.attempts WHERE id = aid) THEN
    RETURN;
  END IF;
  SELECT a.test_version_id, a.duration_minutes, v.total_points INTO asg
    FROM app.assignments a JOIN app.test_versions v ON v.id = a.test_version_id
   WHERE a.id = pg_temp.sid(7, assignment_n);
  started := now() - started_ago;
  submitted := CASE outcome WHEN 'in_progress' THEN NULL
                            WHEN 'timed_out' THEN started + asg.duration_minutes * interval '1 minute'
                            ELSE started + spent_minutes * interval '1 minute' END;
  at_status := CASE outcome WHEN 'in_progress' THEN 'in_progress' WHEN 'timed_out' THEN 'timed_out' ELSE 'submitted' END;

  INSERT INTO app.attempts
    (id, assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
     started_at, deadline_at, submitted_at, flagged, focus_loss_count, score_total)
  VALUES (aid, pg_temp.sid(7, assignment_n), asg.test_version_id, student, 1, at_status, session,
          abs(hashtext(aid::text)), sha256('dev-seed-beacon'::bytea),
          started, started + asg.duration_minutes * interval '1 minute', submitted, flagged, focus_losses,
          CASE WHEN outcome = 'in_progress' THEN NULL ELSE asg.total_points END);

  FOR q IN
    SELECT f.id, f.type, f.points, f.sample_answer, f.prompt
      FROM app.test_version_questions f
      JOIN app.test_version_sections s ON s.id = f.test_version_section_id
     WHERE s.test_version_id = asg.test_version_id
     ORDER BY s.ordinal, f.ordinal
  LOOP
    position := position + 1;
    EXIT WHEN position > answered;
    is_right := position <= right_count;
    manual := q.type = 'short_answer';
    payload := CASE q.type
      WHEN 'short_answer' THEN jsonb_build_object('type', 'text', 'value',
        CASE WHEN is_right THEN coalesce(q.sample_answer, 'The plan shows how the ground floor changes. Overall, the reading room becomes smaller and a new computer area appears.')
             ELSE 'I am not sure.' END)
      WHEN 'fill_blank' THEN jsonb_build_object('type', 'fill_blank', 'values', (
        SELECT jsonb_object_agg(b.id::text, CASE WHEN is_right THEN (SELECT min(a.answer) FROM app.test_version_blank_answers a WHERE a.test_version_blank_id = b.id) ELSE 'banana' END)
          FROM app.test_version_blanks b WHERE b.test_version_question_id = q.id))
      ELSE jsonb_build_object('type', 'choice', 'optionIds', (
        SELECT coalesce(jsonb_agg(o.id::text ORDER BY o.ordinal), '[]'::jsonb)
          FROM (SELECT o.id, o.ordinal FROM app.test_version_options o
                 WHERE o.test_version_question_id = q.id
                   AND CASE WHEN is_right THEN o.is_correct
                            ELSE o.ordinal = (SELECT min(w.ordinal) FROM app.test_version_options w
                                               WHERE w.test_version_question_id = q.id AND NOT w.is_correct) END) o))
    END;
    INSERT INTO app.attempt_answers
      (attempt_id, question_id, payload, requires_manual, auto_score, manual_score, grader_comment, graded_by, graded_at, updated_at)
    VALUES (aid, q.id, payload, manual,
            CASE WHEN manual OR outcome = 'in_progress' THEN NULL WHEN is_right THEN q.points ELSE 0 END,
            CASE WHEN manual AND graded_manually THEN
                   CASE WHEN q.points < 3 THEN CASE WHEN is_right THEN q.points ELSE 0 END
                        WHEN is_right THEN round(q.points * 0.7 * 2) / 2 ELSE round(q.points * 0.4 * 2) / 2 END END,
            CASE WHEN manual AND graded_manually AND q.points >= 3 THEN
                   CASE WHEN is_right THEN 'Bố cục rõ ràng; cần thêm số liệu so sánh.' ELSE 'Thiếu ý chính; xem lại cách mở bài.' END END,
            CASE WHEN manual AND graded_manually THEN pg_temp.lan() END,
            CASE WHEN manual AND graded_manually THEN submitted + interval '1 day' END,
            coalesce(submitted, now()));
  END LOOP;

  IF outcome <> 'in_progress' THEN
    SELECT coalesce(sum(final_score), 0) INTO earned FROM app.attempt_answers WHERE attempt_id = aid;
    IF NOT EXISTS (SELECT 1 FROM app.attempt_answers WHERE attempt_id = aid AND requires_manual AND manual_score IS NULL)
       AND outcome IN ('graded') THEN
      at_status := 'graded';
    END IF;
    UPDATE app.attempts
       SET score_earned = earned, status = at_status,
           graded_at = CASE WHEN at_status = 'graded' THEN
                         CASE WHEN EXISTS (SELECT 1 FROM app.attempt_answers WHERE attempt_id = aid AND requires_manual)
                              THEN submitted + interval '1 day' ELSE submitted END END
     WHERE id = aid;
  END IF;

  IF flagged THEN
    INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at, received_at, client_seq)
    SELECT aid, session, e.kind, started + e.at, started + e.at, e.seq
      FROM (VALUES
        (0, 'fullscreen_exit',  interval '12 minutes'),
        (1, 'fullscreen_enter', interval '12 minutes 20 seconds'),
        (2, 'tab_hidden',       interval '25 minutes'),
        (3, 'tab_visible',      interval '25 minutes 40 seconds'),
        (4, 'fullscreen_exit',  interval '38 minutes'),
        (5, 'fullscreen_enter', interval '38 minutes 15 seconds')
      ) AS e(seq, kind, at);
  END IF;
END;
$$;

-- Mid-term Reading Mock, IELTS 6.5 Evening: eighteen handed in, six of them
-- waiting for the two short answers to be graded, one flagged, two under way.
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 2),  'pending',     8, 6, 52, interval '5 hours',        3, true);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 1),  'graded',      8, 7, 47, interval '8 hours',        0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 3),  'in_progress', 4, 3, 0,  interval '28 minutes',     0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 4),  'pending',     8, 5, 58, interval '1 day 2 hours',  1, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 5),  'graded',      8, 7, 44, interval '1 day',          0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 7),  'timed_out',   8, 4, 60, interval '1 day 6 hours',  0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 8),  'graded',      8, 8, 41, interval '1 day 12 hours', 0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(1, 9),  'in_progress', 2, 2, 0,  interval '12 minutes',     0, false);
SELECT pg_temp.put_attempt(1, pg_temp.sid(2, n),  CASE WHEN n <= 3 THEN 'pending' ELSE 'graded' END,
                           8, 4 + n % 4, 35 + n * 2, interval '1 day' + n * interval '2 hours', 0, false)
  FROM generate_series(1, 12) AS n;

-- Unit 4 · Listening, IELTS Foundation A: nine of sixteen, two flagged.
SELECT pg_temp.put_attempt(2, pg_temp.sid(1, 3),  'graded', 5, 4, 24, interval '2 days',  0, false);
SELECT pg_temp.put_attempt(2, pg_temp.sid(1, 4),  'graded', 5, 3, 28, interval '2 days',  4, true);
SELECT pg_temp.put_attempt(2, pg_temp.sid(2, n),  'graded', 5, 2 + n % 4, 22 + n % 6, interval '2 days' + n * interval '1 hour',
                           CASE WHEN n = 17 THEN 3 ELSE 0 END, n = 17)
  FROM generate_series(15, 21) AS n;

-- Vocabulary Quiz: Travel, TOEIC 600 Weekend and Kids Starters B: eleven.
SELECT pg_temp.put_attempt(3, pg_temp.sid(1, 5),  'graded', 6, 6, 14, interval '4 days',  0, false);
SELECT pg_temp.put_attempt(3, pg_temp.sid(1, 8),  'graded', 6, 6, 12, interval '4 days',  0, false);
SELECT pg_temp.put_attempt(3, pg_temp.sid(2, n),  'graded', 6, 3 + n % 4, 12 + n % 7, interval '3 days' + n * interval '1 hour', 0, false)
  FROM generate_series(29, 37) AS n;

-- Writing Task 1 Practice, closed: all twenty-four handed in, eight waiting.
SELECT pg_temp.put_attempt(5, pg_temp.sid(1, n),  CASE WHEN n IN (5, 8) THEN 'pending' ELSE 'graded' END,
                           2, 1 + n % 2, 38 + n, interval '12 days' + n * interval '3 hours', 0, false)
  FROM generate_series(1, 10) AS n;
SELECT pg_temp.put_attempt(5, pg_temp.sid(2, n),  CASE WHEN n <= 6 THEN 'pending' ELSE 'graded' END,
                           2, 1 + n % 2, 36 + n, interval '11 days' + n * interval '3 hours', 0, false)
  FROM generate_series(1, 14) AS n;

-- ----------------------------------------------------------------- imports
-- Four imports, one per state a teacher meets: pasted text in review, a file
-- committed to the TOEIC draft, a file that failed and a pair of files being
-- processed. Rows only: no source file, artifact or evidence exists in the
-- import bucket, so the review page shows the draft but its source pane has
-- nothing to read (README.md, "Media objects" and "Imports").
CREATE FUNCTION pg_temp.dq_choice(src uuid, qid text, label text, prompt text, opts text[], right_pos int, points text)
RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
  SELECT jsonb_build_object(
    'id', qid, 'label', label, 'type', 'single_choice', 'prompt', pg_temp.doc(prompt),
    'options', (SELECT jsonb_agg(jsonb_build_object('id', qid || 'o' || i, 'label', chr(64 + i), 'content', pg_temp.doc(opts[i])) ORDER BY i)
                  FROM generate_series(1, array_length(opts, 1)) AS i),
    'blanks', '[]'::jsonb,
    'answer', jsonb_build_object('state', CASE WHEN right_pos > 0 THEN 'known' ELSE 'unknown' END,
                                 'optionIds', CASE WHEN right_pos > 0 THEN jsonb_build_array(qid || 'o' || right_pos) ELSE '[]'::jsonb END,
                                 'evidence', '[]'::jsonb),
    'points', points,
    'origins', jsonb_build_object('type', 'inferred_structure', 'prompt', 'source_explicit', 'options', 'source_explicit',
                                  'answer', CASE WHEN right_pos > 0 THEN 'source_explicit' ELSE 'defaulted' END, 'points', 'defaulted'),
    'source', jsonb_build_array(jsonb_build_object('sourceId', src, 'blockId', 'b-' || qid, 'start', 0, 'end', char_length(prompt))))
$$;

CREATE FUNCTION pg_temp.dq_blank(src uuid, qid text, label text, before text, after text, accepted text, points text)
RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
  SELECT jsonb_build_object(
    'id', qid, 'label', label, 'type', 'fill_blank',
    'prompt', jsonb_build_object('format', 'semantic_v1', 'blocks', jsonb_build_array(
      jsonb_build_object('type', 'paragraph', 'content', jsonb_build_array(
        jsonb_build_object('type', 'text', 'text', before, 'marks', '[]'::jsonb),
        jsonb_build_object('type', 'gap', 'id', 'g' || label, 'label', label),
        jsonb_build_object('type', 'text', 'text', after, 'marks', '[]'::jsonb))))),
    'options', '[]'::jsonb,
    'blanks', jsonb_build_array(jsonb_build_object('gapId', 'g' || label, 'label', label, 'accepted', jsonb_build_array(accepted), 'caseSensitive', false)),
    'answer', jsonb_build_object('state', 'known', 'optionIds', '[]'::jsonb, 'evidence', '[]'::jsonb),
    'points', points,
    'origins', jsonb_build_object('type', 'inferred_structure', 'prompt', 'source_explicit', 'options', 'source_explicit',
                                  'answer', 'source_explicit', 'points', 'defaulted'),
    'source', jsonb_build_array(jsonb_build_object('sourceId', src, 'blockId', 'b-' || qid, 'start', 0, 'end', char_length(before || after) + char_length(label))))
$$;

CREATE FUNCTION pg_temp.dsection(id text, title text, instructions text, questions jsonb)
RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
  SELECT jsonb_build_object('id', id, 'title', title, 'instructions', instructions, 'origin', 'inferred_structure',
    'items', (SELECT jsonb_agg(jsonb_build_object('question', q) ORDER BY o) FROM jsonb_array_elements(questions) WITH ORDINALITY AS t(q, o)),
    'source', '[]'::jsonb)
$$;

-- put_import writes an import, its source set, its one run and the run's
-- events, and, when a draft is given, the draft; a committed import also gets
-- its commit record. sources is a jsonb array of role, filename, format,
-- bytes, characters and digest; the first source's id is sid(8, n + 16) and
-- the second's sid(8, n + 32). run gives status, stage, error and the events.
CREATE FUNCTION pg_temp.put_import(n int, title text, status text, revision int, created_ago interval,
                                   sources jsonb, run jsonb, draft jsonb, counts int[], commit_test int)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  iid uuid := pg_temp.sid(8, n);
  run_id uuid := pg_temp.sid(8, n + 48);
  created timestamptz := now() - created_ago;
  running boolean := run->>'status' = 'running';
  terminal boolean := run->>'status' IN ('succeeded', 'failed', 'cancelled');
  worker uuid := pg_temp.did('worker:' || n);
  event record;
  at int := 0;
BEGIN
  IF EXISTS (SELECT 1 FROM app.word_imports WHERE id = iid) THEN
    RETURN;
  END IF;

  WITH imp AS (
    INSERT INTO app.word_imports (id, created_by, request_id, title, status, revision, source_revision, created_at, updated_at)
    VALUES (iid, pg_temp.lan(), pg_temp.did('import-request:' || n), title, status, revision, 1, created, created + interval '10 minutes')
  ), sets AS (
    INSERT INTO app.word_import_source_sets (import_id, revision, created_by, created_at)
    VALUES (iid, 1, pg_temp.lan(), created)
  ), src AS (
    INSERT INTO app.word_import_sources
      (id, import_id, upload_id, expected_revision, role, filename, format, characters, bytes, checksum_sha256,
       storage_key, uploaded_by, ready, source_revision, created_at)
    SELECT pg_temp.sid(8, n + 16 + 16 * (o - 1)::int), iid, pg_temp.did('upload:' || n || ':' || o), 1,
           s->>'role', s->>'filename', s->>'format', (s->>'characters')::int, (s->>'bytes')::bigint,
           sha256(convert_to(s->>'digest', 'UTF8')),
           'originals/' || iid || '/' || pg_temp.sid(8, n + 16 + 16 * (o - 1)::int),
           pg_temp.lan(), true, 1, created
      FROM jsonb_array_elements(sources) WITH ORDINALITY AS t(s, o)
  )
  INSERT INTO app.word_import_source_set_items (import_id, revision, role, source_id)
  SELECT iid, 1, s->>'role', pg_temp.sid(8, n + 16 + 16 * (o - 1)::int)
    FROM jsonb_array_elements(sources) WITH ORDINALITY AS t(s, o);

  INSERT INTO app.word_import_runs
    (id, import_id, source_revision, request_id, requested_by, expected_revision, pipeline_version, status, stage,
     attempt_count, max_attempts, claim_token, worker_id, lease_until, available_at, error_code, result,
     created_at, updated_at, completed_at)
  VALUES (run_id, iid, 1, pg_temp.did('run-request:' || n), pg_temp.lan(), 2, 'word-pipeline-v3',
          run->>'status', run->>'stage', 1, 3, 1,
          CASE WHEN running THEN worker END, CASE WHEN running THEN now() + interval '10 years' END,
          CASE WHEN run->>'status' = 'queued' THEN now() + interval '10 years' ELSE created + interval '1 minute' END,
          run->>'error',
          CASE WHEN run->>'status' = 'succeeded' THEN jsonb_build_object(
            'version', 'word-pipeline-v3', 'candidateSetId', pg_temp.did('candidate:' || n),
            'sources', (SELECT jsonb_agg(jsonb_build_object(
               'sourceId', pg_temp.sid(8, n + 16 + 16 * (o - 1)::int), 'identity', encode(sha256(convert_to(s->>'digest', 'UTF8')), 'hex'),
               'role', s->>'role', 'normalizationSetId', pg_temp.did('normalization:' || n || ':' || o),
               'extractionSetId', pg_temp.did('extraction:' || n || ':' || o)))
              FROM jsonb_array_elements(sources) WITH ORDINALITY AS t(s, o))) END,
          created + interval '1 minute', created + interval '6 minutes',
          CASE WHEN terminal THEN created + interval '6 minutes' END);

  FOR event IN SELECT e FROM jsonb_array_elements(run->'events') AS t(e) LOOP
    at := at + 1;
    INSERT INTO app.word_import_run_events (run_id, import_id, kind, stage, claim_token, attempt_count, worker_id, actor_id, error_code, created_at)
    VALUES (run_id, iid, event.e->>'kind', event.e->>'stage', (event.e->>'claim')::int, (event.e->>'attempt')::int,
            CASE WHEN (event.e->>'claim')::int > 0 THEN worker END,
            CASE WHEN event.e->>'kind' = 'queued' THEN pg_temp.lan() END,
            event.e->>'error', created + at * interval '1 minute');
  END LOOP;

  IF draft IS NOT NULL THEN
    INSERT INTO app.word_import_drafts (import_id, revision, run_id, body, open_action_count, open_confirm_count, created_at, updated_at)
    VALUES (iid, 1, run_id, draft, counts[1], counts[2], created + interval '6 minutes', created + interval '6 minutes');
  END IF;
  IF commit_test IS NOT NULL THEN
    INSERT INTO app.word_import_commits (import_id, request_id, draft_revision, digest, test_id, committed_by, committed_at)
    VALUES (iid, pg_temp.did('commit-request:' || n), 1, sha256(convert_to(draft::text, 'UTF8')), pg_temp.sid(5, commit_test),
            pg_temp.lan(), created + interval '30 minutes');
  END IF;
END;
$$;

-- The deck's pasted sample. Questions 4 and 7 have no key, and the passage
-- before 5 to 7 is text no question owns.
SELECT pg_temp.put_import(1, 'Unit 5 Quick Check', 'needs_review', 5, interval '40 minutes',
  jsonb_build_array(jsonb_build_object('role', 'exam', 'filename', 'pasted-text.txt', 'format', 'text',
                                       'bytes', 1186, 'characters', 1186, 'digest', 'seed-08-pasted-unit-5-quick-check')),
  $r${"status":"succeeded","stage":"ready","events":[
    {"kind":"queued","stage":"queued","claim":0,"attempt":0},
    {"kind":"claimed","stage":"source_validation","claim":1,"attempt":1},
    {"kind":"stage","stage":"normalization","claim":1,"attempt":1},
    {"kind":"stage","stage":"extraction","claim":1,"attempt":1},
    {"kind":"stage","stage":"recognition","claim":1,"attempt":1},
    {"kind":"stage","stage":"validation","claim":1,"attempt":1},
    {"kind":"completed","stage":"ready","claim":1,"attempt":1}]}$r$::jsonb,
  jsonb_build_object('version', 'word-draft-v1', 'title', 'Unit 5 Quick Check',
    'sections', jsonb_build_array(
      pg_temp.dsection('s1', 'Part 1', 'Choose the best answer.', jsonb_build_array(
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q1', '1', 'She ___ in Hanoi since 2019.', ARRAY['lives', 'has lived', 'lived', 'is living'], 2, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q2', '2', 'I ___ him yesterday at the station.', ARRAY['saw', 'have seen', 'see', 'was seeing'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q3', '3', 'They ___ the report yet.', ARRAY['didn’t finish', 'haven’t finished', 'don’t finish', 'hadn’t finished'], 2, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q4', '4', 'How long ___ you known each other?', ARRAY['do', 'did', 'have', 'are'], 0, '1.00'))),
      pg_temp.dsection('s2', 'Part 2', 'Decide if each statement is True, False or Not given.', jsonb_build_array(
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q5', '5', 'Homes near parks always sell for 15 per cent more.', ARRAY['True', 'False', 'Not given'], 2, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q6', '6', 'Parks are cheaper to build than roads.', ARRAY['True', 'False', 'Not given'], 3, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 17), 'q7', '7', 'Trees can cool streets in summer.', ARRAY['True', 'False', 'Not given'], 0, '1.00'))),
      pg_temp.dsection('s3', 'Part 3', 'Write ONE word in each gap.', jsonb_build_array(
        pg_temp.dq_blank(pg_temp.sid(8, 17), 'q8', '8', 'We have lived here ', ' 2015.', 'since', '1.00'),
        pg_temp.dq_blank(pg_temp.sid(8, 17), 'q9', '9', 'She has worked here ', ' three years.', 'for', '1.00'),
        pg_temp.dq_blank(pg_temp.sid(8, 17), 'q10', '10', 'Have you ever ', ' to Da Nang?', 'been', '1.00')))),
    'notices', jsonb_build_array(jsonb_build_object(
      'id', 'notice-s2-passage', 'code', 'UNASSIGNED_SOURCE_TEXT', 'severity', 'review_required', 'target', 's2', 'count', 1,
      'evidence', jsonb_build_array(jsonb_build_object('sourceId', pg_temp.sid(8, 17), 'blockId', 'b-passage-s2', 'start', 0, 'end', 133)))),
    'acknowledged', '[]'::jsonb),
  ARRAY[2, 1], NULL);

-- A file committed to the TOEIC draft test.
SELECT pg_temp.put_import(2, 'TOEIC Part 5 · Incomplete sentences', 'committed', 6, interval '4 days',
  jsonb_build_array(jsonb_build_object('role', 'exam', 'filename', 'TOEIC Part 5 · Incomplete sentences.docx', 'format', 'docx',
                                       'bytes', 48230, 'digest', 'seed-08-toeic-part-5')),
  $r${"status":"succeeded","stage":"ready","events":[
    {"kind":"queued","stage":"queued","claim":0,"attempt":0},
    {"kind":"claimed","stage":"source_validation","claim":1,"attempt":1},
    {"kind":"stage","stage":"normalization","claim":1,"attempt":1},
    {"kind":"stage","stage":"extraction","claim":1,"attempt":1},
    {"kind":"stage","stage":"recognition","claim":1,"attempt":1},
    {"kind":"stage","stage":"validation","claim":1,"attempt":1},
    {"kind":"completed","stage":"ready","claim":1,"attempt":1}]}$r$::jsonb,
  jsonb_build_object('version', 'word-draft-v1', 'title', 'TOEIC Part 5 · Incomplete sentences',
    'sections', jsonb_build_array(
      pg_temp.dsection('s1', 'Part 5 · Incomplete sentences', '', jsonb_build_array(
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q1', '1', 'The manager asked all employees ___ the new safety rules.', ARRAY['to follow', 'follow', 'following', 'followed'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q2', '2', 'Please send the invoice ___ Friday.', ARRAY['by', 'at', 'on', 'in'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q3', '3', 'Ms. Tran is ___ for coordinating the training programme.', ARRAY['responsible', 'respond', 'responsibly', 'response'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q4', '4', 'The conference was postponed ___ the heavy rain.', ARRAY['because of', 'because', 'although', 'despite of'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q5', '5', 'All visitors must ___ at the front desk.', ARRAY['register', 'registered', 'registering', 'registration'], 1, '1.00'),
        pg_temp.dq_choice(pg_temp.sid(8, 18), 'q6', '6', 'The new software is ___ more efficient than the old one.', ARRAY['significantly', 'significant', 'signify', 'significance'], 1, '1.00')))),
    'notices', '[]'::jsonb, 'acknowledged', '[]'::jsonb),
  ARRAY[0, 0], 7);

-- A legacy .doc the pipeline could not read.
SELECT pg_temp.put_import(3, 'IELTS Reading Pack B', 'failed', 5, interval '18 hours',
  jsonb_build_array(jsonb_build_object('role', 'exam', 'filename', 'IELTS Reading Pack B.doc', 'format', 'doc',
                                       'bytes', 912384, 'digest', 'seed-08-ielts-reading-pack-b')),
  $r${"status":"failed","stage":"source_validation","error":"SOURCE_INVALID","events":[
    {"kind":"queued","stage":"queued","claim":0,"attempt":0},
    {"kind":"claimed","stage":"source_validation","claim":1,"attempt":1},
    {"kind":"failed","stage":"source_validation","claim":1,"attempt":1,"error":"SOURCE_INVALID"}]}$r$::jsonb,
  NULL, NULL, NULL);

-- An exam and its answer key being read. The run is queued at the extraction
-- stage and not due for ten years, so no worker claims it and it holds none
-- of the worker's slots: it stays "Processing" for a comparison.
SELECT pg_temp.put_import(4, 'Grade 9 Mock Test 4', 'processing', 4, interval '20 minutes',
  jsonb_build_array(
    jsonb_build_object('role', 'exam', 'filename', 'Grade 9 Mock Test 4.docx', 'format', 'docx', 'bytes', 1258291, 'digest', 'seed-08-grade-9-mock-test-4'),
    jsonb_build_object('role', 'answer_key', 'filename', 'Answer key Tests 1–5.docx', 'format', 'docx', 'bytes', 88064, 'digest', 'seed-08-answer-key-tests-1-5')),
  $r${"status":"queued","stage":"extraction","events":[
    {"kind":"queued","stage":"queued","claim":0,"attempt":0},
    {"kind":"claimed","stage":"source_validation","claim":1,"attempt":1},
    {"kind":"stage","stage":"normalization","claim":1,"attempt":1},
    {"kind":"stage","stage":"extraction","claim":1,"attempt":1}]}$r$::jsonb,
  NULL, NULL, NULL);

COMMIT;
