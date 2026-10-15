# Quizzivy — Frontend Portal & Data Model Specification

**Version:** 0.112 · **Owner:** Thuong · **Audience:** AI coding agent + future contributors
**Scope:** web frontend (admin + student portals) and the PostgreSQL data model. Go backend implementation is a separate spec; the API surface in §15 is the contract both sides implement.

**Changes pending (R5, v0.12.0)**

- T-R5.15 §15 Three operations under `/admin/data-exports`, all `system.data_export` (Admin only): `requestDataExport` (`POST`, `{includeAnswers}` → `202 DataExport`; `409 EXPORT_IN_PROGRESS` while any export is queued or running; `429 RATE_LIMITED` with `Retry-After` while the caller has an export from the last hour that has not failed), `getDataExport` (`GET /:id`, the requester's own export, another's is `404`) and `downloadDataExport` (`POST /:id/download` → `{url, expiresInSeconds: 60}`, a presigned attachment named `quizzivy-export-YYYYMMDD.zip`; `404` for another admin's export, `410 EXPORT_UNAVAILABLE` unless the export is ready, unexpired and signed fewer than three times). A download is a signing and not a transfer, because the browser reads the object from storage and the server never learns whether it finished, so a ready export can be signed up to three times in its 24 hours (D-024): each signing is a fresh 60-second URL with its own `data_export.downloaded` audit row, `DataExport.downloadedAt` keeps the first and `DataExport.downloads` (0 to 3) counts them. Three new error codes, `EXPORT_IN_PROGRESS`, `EXPORT_UNAVAILABLE` and `EXPORT_OFF`. The hour belongs to the export and not to the transport (D-021): a failed export, a request refused with `409` and a request refused with `429` do not count, and the check runs in the transaction that inserts. Rate limits per actor in front of that: 10 requests a minute, 60 reads a minute (600 an hour), 10 downloads an hour. Without import storage and the worker the three answer `501 EXPORT_OFF`, as the AI operations answer `AI_OFF`.
- T-R5.15 §13 `app.data_exports` (`00134_create_data_exports.sql`): one row for each request. A partial unique index allows one export queued or running in the whole centre, so two Admins asking at once cannot both build a ZIP of everyone's data. The ZIP stays in the private import bucket at `exports/{id}.zip` for 24 hours and is fetched once, by its requester, from the app page the email links to; nothing in the email or the table is a credential.
- T-R5.15 §15 The worker builds the ZIP from one `REPEATABLE READ`, read-only snapshot, so every file describes the same moment: `users.csv` (no hashes or tokens), `roles.csv`, `classes.csv` (no join codes), `class_members.csv`, `tests.csv`, `test_versions.csv`, `assignments.csv`, `attempts.csv` (no session or beacon secrets), `answers.csv` when asked, and a `README.txt` in Vietnamese then English that describes every column. Every CSV streams through `platform/tabular` (byte-order mark, CRLF, a cell that begins `=`, `+`, `-` or `@` is prefixed with `'`), so memory stays bounded: 50,000 attempts and 50,000 answers grew the live heap by 8 MiB. A claim lasts 30 minutes, a build is cut off after 25, and an export interrupted three times is failed with `WORKER_INTERRUPTED`. A failure records a code, never a message, and sends nothing. An archive a killed worker left in the work directory is removed the next time the worker finds nothing to claim.
- T-R5.15 §15 The audit log gains `data_export.requested` and `data_export.downloaded` (entity `data_export`, listed under All only), with their `vi` and `en` sentences.
- T-R5.16 §13 A daily housekeeping runs in the API at start-up and then on the 24-hour tick of the refresh-token prune, so it adds no wake of its own. Each step removes at most 500 rows or objects a batch, repeats while a batch comes back full (20 rounds and a minute a step), logs counts and, for a failure, its class and never the error's text, and a step that fails or runs out of time does not stop the others; the next run picks up where it stopped. It deletes `user_activity_days` rows older than 400 days (counted from the start of the local day in `APP_TIMEZONE`), outbox rows sent or failed more than 30 days ago (a pending row never), set-password links used, revoked or expired more than 30 days ago (a link that still works never), and the `branding/` logo objects that neither the light nor the dark logo names and that were written more than 24 hours ago.
- T-R5.16 §15 An export past its 24 hours is expired in the table and not only on read: the housekeeping deletes its object from the imports bucket, then sets the row `expired` with no `storage_key`. The row stays, with its size, its time and its download count. A missing object is not an error, so a run that stopped between the two steps finishes on the next.
- T-R5.5 to T-R5.7: §5.6 (new) The centre's one settings document: profile, two logos, sign-in policy and retention choices, saved under a revision and audited as `organization.updated`; the logo rules; what each sign-in switch does; the lockout judgement; the 30-second propagation.
- T-R5.5 to T-R5.7: §15 `GET`/`PUT /admin/settings`, `POST /admin/settings/sign-in-impact` and `POST /admin/settings/logo` (`system.settings.write`), and the two open reads `GET /public/organization` and `GET /public/organization/logo`. That makes nine open operations, not seven. New error codes `SIGN_IN_POLICY_LOCKOUT`, `PASSWORD_SIGN_IN_DISABLED`, `GOOGLE_SIGN_IN_DISABLED` and `GOOGLE_DOMAIN_NOT_ALLOWED`.
- T-R5.5 to T-R5.7: §13.5 `app.organization_settings`, one row (`00130_create_organization_settings.sql`): the app role may read and update it and cannot insert or delete it.
- T-R5.5 to T-R5.7: §5.6 A name holds no line break, control or invisible character; a website is a domain or an `https` address without credentials; a contact email is an address with a domain; a save that changes nothing writes nothing and keeps its revision.
- T-R5.5 to T-R5.7: §5.6, §12 The organization logo is stored and served as a PNG only (D-019). The server answers an SVG with `415 MEDIA_TYPE_UNSUPPORTED`; the settings page draws an SVG the Admin picks to a 1024 x 1024 canvas in the browser and uploads that PNG. The `svg` logo type, `branding/{sha256}.svg` and the sandbox header for SVG are gone.
- T-R5.7 §6.4 While the centre has turned joining by code off, the join-code card and the Classes list disable (not hide) the "Students can join" switch and New code or Create code with the centre's sentence as the reason, mark an existing code inactive, and stop promoting the code in the list's page lead; the callout is a row of its own above the cards and links a `system.settings.write` holder to the sign-in settings, and says to ask the admin otherwise (D-023).
- T-R5.7 §6.4 D-028: the rule reaches everything that hands a code on or invites its use. While joining by code is off, Copy code, Copy link, Download QR, the QR and the card's and menu's Copy link and Show QR are disabled with the same reason; New class creates the class without a code and its toast says why; the New class hint, the empty Classes list and a class's empty Members text stop pointing to a code; an expired or used-up code reads "Inactive" with the reason beside it. The query that tells a teacher's screen whether joining is off asks the browser to revalidate and is stale after 30 seconds, so the setting an Admin just changed reaches an open page in a moment, not minutes, and it follows the setting again when the tab is focused after 30 seconds (D-030). The replaced-codes notice and a code that cannot be read stop telling the teacher to send or make a new code while joining is off, and the card's Copy link and Show QR icons carry the reason as their title. A paused control draws its colours at half strength, not the element, so its focus ring stays whole.
- T-R5.7 §12 D-029: a control paused with `aria-disabled` fades its colours, not the element, so its focus ring keeps `--focus` at full strength; `FormDialog`'s submit, `BulkActions` and `DirtyBar` follow it, and a ratchet test (`paused-opacity.test.ts`) holds the other sites at an exact count per file, whichever way the opacity is written on an element that declares `aria-disabled`, until the sweep after R5 removes them; the submit buttons of Change password, Set password and Add member fade their colours now.
- T-R5.7 §6.4 QC-U-30: with joining by code off, the new-code note beside the paused button is hidden, and the archived-class advice, the member-remove confirm and the Students remove-all confirm no longer send a teacher to make, share or rejoin with a code.
- T-R5.12 §13.4 `GET /admin/audit-log` and `GET /admin/audit-log/export` (`system.audit.read`) read `audit_log` through `modules/audit`, newest first over `audit_log_recent_idx`, with the tab counts, the ranges in the viewer's stored zone and a CSV or JSON export of at most 100,000 entries that is itself audited before its first byte.
- T-R5.12 §13.4 A key whose name, compared without case or punctuation, contains `password`, `passcode`, `token`, `secret`, `authorization`, `credential`, `cookie`, `ciphertext`, `apikey`, `privatekey`, `accesskey` or `signingkey`, or is exactly `otp` or `pin`, is dropped from an audit entry's details and from exports, at any depth.
- T-R5.12 §13.4 Migration `00133` builds `audit_log_recent_idx` after dropping any invalid index of that name, so a rerun after an interrupted build repairs it.
- T-R5.18 §15 Three read-only Admin operations, no migration: `getAdminOverview` and `getAdminSummary` (`workspace.admin`) and `getUsage` (`system.settings.write`). A section the caller's keys do not open is `null` and costs no query: `people` and `dailyActive` need `people.users.manage`, `classes` `scope.all`, `recentChanges` `system.audit.read`, `storage` `system.settings.write` and the credit-request row `ai.credits.manage`.
- T-R5.18 §13.8 The Admin's readings count the whole centre under no scope. A day is a local date in `APP_TIMEZONE`: the chart is today and the thirteen days before it, and `activeThisWeek` the seven days ending today, both counting an account that was active on the day even if it has been disabled since. A teacher is an enabled account that is not a student target of `app.student_like_roles`, an Admin and a custom role included; `teachersDisabled` is the same over disabled accounts and a seat is an account that is not disabled. The storage row appears when a meter with a limit reads 80% or more, rounded half up as `attention[].detail` quotes it. The plan is six optional `PLAN_*` variables, validated at start-up (the numbers are plain digits) and never enforced (DG-55).
- T-R5.19 §15 `downloadOpenApi` serves `api/openapi.yaml` converted to JSON unchanged; `getApiReferenceSummary` reports the embedded contract's versions, operation count and count under each tag.
- T-R5.20 §2, §3, §5.4 The Admin console's shell and route tree (its screens are later R5 tasks; each route renders a placeholder until its task lands). Four route trees: public, `/teacher/*`, `/admin/*` and `/app/*`. `AdminWorkspace` holds `/admin` for the `admin` workspace, `RequirePermission` reads the `permission` of the leaf route's handle and answers the 403 page when the user lacks it (a custom role holding only `system.audit.read` gets 403 at `/admin/users`), and `AdminLayout` is the shell. A `/admin/*` address that is not a console route still redirects to the same path under `/teacher`; a console address answers the 403 page to a user without the `admin` workspace, except `/admin` and `/admin/classes`, whose `/teacher` twins are real pages and keep redirecting.
- T-R5.20 §5.4 `homePathFor` sends a built-in Admin (the holder of `scope.all`) to the console in `preferences.workspace` and to `/admin` the first time, any other holder of the `teacher` workspace to `/teacher`, a user with only the `admin` workspace to `/admin`, and anyone else to `/app`.
- T-R5.20 §7, §15 `UserPreferences.workspace` (`teacher` or `admin`), written by `PATCH /me/preferences` when the user switches console; any other value answers `400 VALIDATION_FAILED`. No new operation and no migration: the preferences are one JSON document.
- T-R5.20 §8 The consoles share one shell. The workspace switcher and the account menu gain the Admin console for a holder of both; the Admin menu has System settings, API reference, Teacher workspace, the theme and Sign out, and for a user without the `teacher` workspace also Change password (when the account has one) and Language. `/change-password` asks for the current password when the change is not forced, is titled "Change password" then, names the current-password field when it is empty, confirms the change with a toast, and goes back to the page the account menu was opened from (`?next=`).
- T-R5.20 (D-022.3) §7, §15 `CurrentUser.role` returns, as a different thing from the v0.9.x `role` string that stays refused: an optional `CurrentUserRole` (`id`, `builtinKey`, `name`) read with the user from `app.roles`, on the caller's own document only (every operation that answers `CurrentUser`: sign-in, Google sign-in, setting a password from a link, `GET`/`PATCH /auth/me`, linking Google and the avatar writes; `POST /auth/refresh` answers a token and no user); `User`, and so any payload about someone else, carries no role. No migration, no new operation and no new permission; the access module and the principal cache are untouched.
- T-R5.20 (D-022.3) §8 The sidebar footer's second line is the user's role name, as the deck draws it under the name. A built-in role reads from `role.builtin.<key>` (`vi` and `en`) while its stored name is still the seeded Vietnamese one, and as stored once an Admin renamed it; a custom role reads as stored. English therefore reads "Admin" for the built-in administrator, the Roles screen's word, not the deck's "Administrator". A response without a role draws the name alone. The line is read when the user's document is, so a rename of the user's own role shows after the next reload or `PATCH /auth/me`, not live. `lib/roleName.ts` is the one place that rule lives; the Roles screen (T-R5.24) reuses it.
- T-R5.28a §5.6 System settings is built at `/admin/settings/:section` with Organization and Sign-in: one form and one unsaved-changes bar over the whole document, the sections kept mounted, a save at the revision the form was last reset from (`STALE_WRITE` offers Reload), a server-refused field shown on its field with its section opened, a save tried even when the browser reports no connection (it fails with a message and does not wait), and a read of `GET /public/organization` past the browser's cache after every save, and whatever was typed while a save was in flight kept as an unsaved change. Data & privacy and Plan & usage are T-R5.28b. No operation, schema or migration changes.
- T-R5.28a §5.6 The Sign-in section: four switches, the Google domains as chips (Enter, comma or space adds; lower-cased, `@` stripped, deduplicated; at most 253 characters each; Backspace removes the last), the debounced sign-in impact as a warning, and a lockout (no method, or the server's count) that pauses Save changes with its reason; a `422 SIGN_IN_POLICY_LOCKOUT` from a save does the same until the policy changes.
- T-R5.28a §5.6, §12 The logo dialog refuses on its field a file that is not a PNG or an SVG, a PNG over 1 MiB and an SVG that does not draw or is not square, before anything is sent; an SVG is drawn to a 1024 x 1024 canvas and uploaded as a PNG (D-019); the server's refusal of a file shows its message on that field. The page has no control that removes a logo (DG-41).
- T-R5.28a §12 `uploadFile` takes `fields`, sent before the file in the same multipart form (the logo's `variant`). `DirtyBar` takes `blocked`, a reason that pauses Save changes alone and reads in the sentence's place, and `errorAction`, a button beside Discard while the error shows. `SettingsLayout` takes `footer`, which follows the sections in their column. `ChipInput` takes `spaceAdds`, `resolve` and `id`. `FormDialog` reveals field errors after any submit, valid or not, so a rule the caller finds while handling a submit shows on its field.
- T-R5.29 §12 The front door reads `GET /public/organization` through `useOrganization()`: a five-minute stale time, never blocking a render and never throwing, each line the centre's own value, else its `VITE_ORG_*` build value, else hidden, and every way in on when there is no answer. The answer is read member by member, so a wrong shape counts as no answer. The brand panel and the system pages' caption print the centre's name and logo, light or dark by theme (a logo made for the other theme sits on a tile of the opposite ground, and one that will not load is left out), and the name the settings row is born with, `Quizzivy`, counts as unset.
- T-R5.29 §12 Sign-in hides the password form when password sign-in is off and the Google button when Google is off or not configured, says no way in is available when none is left, and links "Have a class code?" and names a remembered class only while joining with a code is on. Forgot password shows the front desk's phone and hours, shows Continue with Google only where sign-in offers it, and sends a visitor to sign-in when password sign-in is off. The join page says joining with a code is off in place of the form and forgets the remembered class (DG-40, DG-41).
- T-R5.3 §5.4 An account sets or resets its password from an emailed link: a 256-bit token in the URL fragment, of which the database keeps the SHA-256, valid 24 hours and once. An unknown, used, expired or revoked link, and the link of a disabled account, all answer one `410 SET_PASSWORD_LINK_INVALID`. Using a link sets the password, clears `mustChangePassword`, ends the account's other links and sessions and signs the account in. Disabling an account revokes its unused links with its sessions, and enabling it again restores none; an Admin sends a new link through Reset. A Google-only account asks for its own link from Settings and keeps its session and its Google link. `/set-password` joins the public tree.
- T-R5.3 §5.4 The set-password page waits up to three seconds for the centre's public answer before it shows the form, shows the passwords-off state when that answer says so, and keeps a form it has shown; Settings withdraws the set-password control once the answer says off. The answer is revalidated and kept for thirty seconds, so a switch an Admin just changed reaches a person holding a link in a moment. The page's heading takes focus in every state.
- T-R5.3 §6.5 `checkSetPasswordLink` and `setPasswordWithLink` are open and limited per address and per token; `requestSetPasswordLink` is limited per signed-in user. The set of operations that need no bearer token is now eleven, and the bodies keyed on a field are six.
- T-R5.3 §13.3 `app.set_password_tokens` (`00135_create_set_password_tokens.sql`, `docs/plan/20-data-model.md` D-55, §53).
- T-R5.3 §15 Three operations: `POST /auth/set-password/check`, `POST /auth/set-password` and `POST /auth/set-password/request`, and the error code `SET_PASSWORD_LINK_INVALID`. Two audit actions: `user.password_set` and `user.password_link_requested`.
- T-R5.9a §15 `createUser` (`POST /admin/users`, `people.users.manage`) adds an account of any role the caller may give: `{fullName, email, roleId, classId?, delivery}` answers `201 {user, temporaryPassword?}` with `Cache-Control: no-store`. The role must be one the caller may give (the Assistant `422 ROLE_NOT_ASSIGNABLE`, a role beyond the caller's own permissions `403 TARGET_ROLE_NOT_ALLOWED`, an unknown one `404`); `classId` only for a student-target role and only with `scope.all`, and never an archived class (`400`/`404`); an email in use in any letter case `409 EMAIL_TAKEN`. `email_link` creates the account with no password and queues the invite in the same transaction, so an invite that cannot be queued takes the account back with it; `temporary_password` shows the password once, marks it for change unless the centre allows keeping it, and answers `422 PASSWORD_SIGN_IN_DISABLED` while password sign-in is off. `created_by` is the actor.
- T-R5.9a §15 `createUser` and a user import read a name the same way (QC-N-3, QC-I-3): composed to NFC, trimmed, each run of white space one space; a name that is then empty, over 200 characters, holds a control, format, bidirectional or filler character or an unassigned code point, or holds no letter, digit, punctuation mark or symbol, is `400 VALIDATION_FAILED` on `fullName` (a file's row is `INVALID` or `REQUIRED`). The role is held against edits from the check to the commit and checked again, up to three times, when it changed in between (`409 STALE_WRITE`) (QC-N-2).
- T-R5.9a §5.4 `createUser` joins the operations that hand out a credential: 30 a minute and 300 an hour per signed-in user, after the permission check.
- T-R5.9a §15 The audit action `user.created {role, roleId, delivery}`, in both languages.
- T-R5.9a §15 `listUsers` (`GET /admin/users`, `people.users.manage`) is the Users table: every account whatever its role, newest first, ten, twenty or fifty to a page (`limit`, twenty by default). The Admins tab is the built-in Admin role, Students the strict student targets of §5, Teachers everyone else, and a disabled account stays in its role's tab; `facets` count all five tabs under the same `q`, `email` and `status`, whichever tab is shown. `q` matches the name and the email together ignoring case and accents, `email` is an exact match in any letter case, and `status` (`must_change_password`, `never_signed_in`) never reaches a disabled account. A row carries the role with its icon and colour, whether the account signs in with a password and with Google, its class count with the first class's name (a student's classes are the ones it has joined, anyone else's the ones it teaches, archived classes never), when it was last active, and its status.
- T-R5.9a §15 `getUser` (`GET /admin/users/:id`, `people.users.manage`) answers any account whatever its role, with who created it, up to 20 of its classes, what it owns (`ownedContent`: every class it teaches, archived ones too, and the tests, questions, media and assignments not deleted), the tests it is taking, up to 8 recent events (the last sign-in and its method, its own audit entries, a student's submissions and class joins) and `can {changeRole, resetPassword, disable, signOut, transfer}`. `can` is the guards' answer for this caller and this account: the subset rule for the first four; additionally never on oneself and, for `changeRole` and `disable`, never on the last active Admin; `changeRole` also needs `people.roles.manage`; `disable` is false for an account already disabled; `transfer` needs `scope.all` and a disabled account.
- T-R5.14 §15 Three operations import accounts from a CSV or XLSX file, all `people.users.manage`, the first two `multipart/form-data` (5 MiB): `previewUserImport` (`POST /admin/user-imports/preview`, `{file, defaultRole: student | teacher, delivery: email_link | sheet}`) answers the counts and writes nothing, `createUserImport` (`POST /admin/user-imports`, the same plus the caller's `requestId`) re-reads and re-judges the file and stores the job with its accepted rows in one transaction, `202 UserImportJob`, and `getUserImport` (`GET /admin/user-imports/:id`) reports the caller's own job; another user's is `404`. The same `requestId` of the same caller returns the first job and creates no second. The columns are `full_name`, `email`, `role` and `class`; a cell is refused with `REQUIRED`, `TOO_LONG`, `INVALID`, `DUPLICATE_IN_FILE`, `FORMULA_LIKE` (an address a spreadsheet would run as a formula, such as one starting with `+` or `=`, which the hand-out sheet could not carry as the address that signs in), `UNKNOWN_ROLE`, `AMBIGUOUS_ROLE`, `ROLE_NOT_ALLOWED` (beyond the importer's permissions), `ROLE_NOT_GIVABLE` (the Assistant), `UNKNOWN_CLASS`, `AMBIGUOUS_CLASS` or `CLASS_NOT_ALLOWED`, an address that already has an account is skipped and never changed, and the file's text is not stored. Eleven error codes: `USER_IMPORT_FILE_TOO_LARGE` (`413`), `USER_IMPORT_FILE_UNREADABLE`, `USER_IMPORT_FILE_EMPTY`, `USER_IMPORT_TOO_MANY_ROWS`, `USER_IMPORT_COLUMNS`, `USER_IMPORT_NOTHING_TO_CREATE`, `CSV_NOT_UTF8`, `XLSX_MACROS` and `XLSX_ENCRYPTED` (`422`), each worded in both languages with the way out, and none carries text of the file. Without the import worker or `SECRETS_KEY` the three answer `501`. Limits per signed-in user: 10 previews a minute and 5 imports an hour.
- T-R5.14 §13.3 `app.user_import_jobs` (`00136_create_user_import_jobs.sql`) and `app.user_import_rows` (`00137_create_user_import_rows.sql`), `docs/plan/20-data-model.md` D-56, D-57, §54.
- T-R5.14 §15 The audit action `user_import.requested {format, delivery, defaultRoleId, rows, existing, invalid}`, in both languages, under Imports (the audit log's Imports tab holds Word and user imports; the accounts a job creates are written as `user.created` under Accounts).
- T-R5.14 §15 The import worker creates the accounts. It takes the oldest queued job (or one still running 30 minutes after its claim) and settles its rows 25 to a transaction; each account is created by the importer, enrolled in its class when the row names one, given an invite when the job is delivered by email link or a temporary password when it is delivered by sheet, and audited as `user.created {via: import, jobId}`. An address that came to belong to an account after the file was read is skipped. The importer's rights are read again at the start of every batch, from the database and not from the API's cache: a disabled account, a lost `people.users.manage`, a role the importer may no longer give or a class without `scope.all` fails the job `IMPORTER_NOT_ALLOWED`, and password sign-in turned off before a sheet job finishes fails it `DELIVERY_UNAVAILABLE`. A job claimed three times without finishing is `WORKER_INTERRUPTED`. The accounts made before a failure stay, `UserImportJob.errorCode` names the reason and carries no text of the file.
- T-R5.14 §15 `downloadUserImportSheet` (`POST /admin/user-imports/{id}/sheet`, `people.users.manage`, the importer only) gives the temporary passwords of a sheet job once: a UTF-8 CSV with a byte-order mark, `no-store`, one line for each account the job created (`full_name`, `email`, `role`, `class`, `temporary_password`), a cell that a spreadsheet would run as a formula starting with `'` (an address of that kind was refused when the file was read, so the email and password cells reach the reader as they sign in), the content type `text/csv; charset=utf-8` and the file's name readable by a page on another origin. A student the worker could not place in the class (it was archived after the file was read) has the class cell blank. The same transaction erases the passwords, so a second request, a job that is not done, was sent by email, created no account or finished more than 24 hours ago answers `410 SHEET_UNAVAILABLE` (new error code), and another user's job is `404`. Audited as `user_import.sheet_downloaded {rows}` without a password. Limited to 5 a minute per signed-in user; `501` without the import worker or `SECRETS_KEY`. The passwords of a failed job are erased, so its accounts need a password reset.
- T-R5.14 §15 The audit action `user_import.sheet_downloaded` is added under Imports, in both languages.
- T-R5.14 §15 The sealed passwords of a sheet nobody downloaded are erased when its 24 hours end, by the daily housekeeping (`import_sheets`), which leaves the job and its rows as they are; the 30-day removal of jobs and rows is T-R5.16's.
- T-R5.9a §13 `users_search_trgm_idx` (`00138_index_users_search.sql`, `docs/plan/20-data-model.md` D-58): a gin trigram index on `app.immutable_unaccent(lower(full_name || ' ' || email))`, the expression the Users search compares with. It serves a search of three characters or more; a shorter one still reads the table. The migration ends with `ANALYZE app.users`, so the planner has statistics for the expression from the first request. No contract change.
- T-R5.24 §12 The Roles & permissions screen (`/admin/roles`, `people.roles.manage`) is a carousel with a card for each role (the Assistant's included), the permission matrix and an unsaved-changes bar. The matrix is an ARIA grid with one tab stop: the arrows move between cells, Home and End to the ends of a row, with Control to the ends of the table. A role header, or "Sửa quyền" on a card, frames that role's column (`?role=<id>`). The bar counts the cells that differ from what the server holds, so a cell turned back is not counted, and says how many test attempts are in progress right now when a role loses Take tests; Save sends one `setRolePermissions` with the revisions read, and a stale save reloads the roles and keeps the unsaved cells marked.
- T-R5.24 §12 A cell the caller cannot change stays focusable and answers `aria-disabled`, with its reason as title and description and its colours, never the element, at half strength. The reasons follow the server's order: the Admin's cells but Take tests, the Student's Take tests, the caller's own role (the Admin's Take tests apart), then the subset rule. The Student's and the caller's own are the two the deck does not draw; a cell the Admin has turned is marked with a ring.
- T-R5.24 §12 New role starts with the permissions of the role chosen under "Sao chép quyền từ", none by default: the Teacher, the Student or a custom role the caller may act on, never the Admin or the Assistant (segments up to four choices, else a list). A name another role goes by, stored or shown, in any letter case, is refused before the request and again when the server answers `ROLE_NAME_TAKEN`. Edit role sends only the fields changed against the card, with the latest revision, so a built-in role's name is stored only when the Admin renames it, and a save that changes nothing sends nothing. A role the subset rule keeps from the caller has its pencil `aria-disabled`. Neither dialog nor card offers Delete (DG-52). `FormDialog` gains the field kinds `icons` and `colors`.
- T-R5.26 §12 The Admin console's audit log is at `/admin/audit` (`system.audit.read`), ten entries a page, newest first, with its category, search, date range (a preset or two custom days) and page in the address. The categories are the segmented control with the count each holds from `facets`, grouped for the language and shown at zero, which follow the search and the range and not the category. A row shows When (without its year, in the viewer's zone), Who (the system for an entry with no actor), Action (the `audit.action` sentence, humanised when the client has none) with a dot for its category, Entity, IP and a chevron, folding When, Entity and IP into the first two columns as the content narrows (700, 860 and 1040 px); it opens to show what it changed, as the old value struck through and the new one, any other detail the entry recorded, and the device. "Reset" clears the search and the range and keeps the category.
- T-R5.26 §12 Export opens a dialog: a period (7 days, 30 days, this year), a format (CSV or JSON) and, while a category or a search is set, "current filters only" (on at first), which narrows the file by those two. The range of the list is not sent; the period is the export's own. A period of more than 100,000 entries asks for a shorter one. `DataTable` gains an `expansion` (a row opens a panel under itself) and a `hideHeader` column, `Segmented` groups the thousands of a count for the language, `Pager` draws no size control for one size, and the dropdown menu gains a radio item.
- T-R5.27 §12 The Admin console's API reference is at `/admin/api` (`system.api_reference`): the contract's OpenAPI and `info.version` and its endpoint count from `getApiReferenceSummary`, a card for each tag of the contract with its name and blurb in both languages (a tag the client has no words for is drawn by its name, and `api-groups.test.ts` fails when the contract has one), the base URL with Copy, the contract as `openapi.json` (the document the API serves, not the deck's `openapi.yaml`) and a button that makes the docs session and opens `/docs` in a new tab, opened in the click and cut off from the page. A group's card opens the reference at `#tag/{tag}`. The note says what is true: the session reads the documentation and does not call the API for the person (DG-10).
- T-R5.27 §12 R4's Settings no longer lists API reference. `/teacher/settings/api` goes to `/admin/api` for who holds `system.api_reference` and to Profile for anyone else.

**Changes since v0.111**

R5, T-R5.8 (second QC follow-up on the roles API: QC-X-13, QC-X-14).

- §15 `createRole` and `updateRole` refuse a name that has no letter, digit, punctuation mark or symbol (a lone combining accent, a variation selector), and, in a name or a description, any character that is not graphic: a control, format, line- or paragraph-separator, private-use or unassigned code point, and the blank fillers the Hangul filler, its two jamo forms, the halfwidth filler and the Braille blank. A description may still hold line breaks, as the Edit role dialog's text area sends them. The refusal names the first offending code point in `details`.
- §15 The 60 characters of a name and the 300 of a description count the text after NFC. The schema's own bound on the text as sent is four times that (240 and 1200), so a name typed decomposed is no longer refused before it is composed.

**Changes since v0.110**

R5, T-R5.8 (QC follow-up on the roles API).

- §15 `createRole` and `updateRole` store a name and a description in Unicode NFC, so the composed and decomposed forms of one name are one name, and refuse a name that holds a line break or a control, format, line- or paragraph-separator character. At most 200 roles exist, the built-in four included; `createRole` past that answers `422 ROLE_LIMIT_REACHED`. `AssignableRole.studentTarget` says a custom role with no permission is a student target. The role operations promise propagation to another machine "within about ten seconds".
- §13 Advisory lock `(73819, 44)` serialises role creation against the limit (`docs/plan/20-data-model.md` §28).

**Changes since v0.109**

R5, T-R5.8 (roles and permissions API, no screen yet).

- §15 Five operations under `/admin/roles`: `listRoles` and `createRole`, `updateRole` (`PATCH /admin/roles/:id`), `setRolePermissions` (`PUT /admin/roles/permissions`), all `people.roles.manage`, and `listAssignableRoles` (`people.users.manage`). A role is never deleted (DG-52). Seven new error codes: `ROLE_NAME_TAKEN`, `ROLE_NOT_ASSIGNABLE`, `ROLE_LOCKED`, `STUDENT_NEEDS_TAKE_TESTS`, `KEY_NOT_GRANTABLE`, `CANNOT_ACT_ON_SELF` and `TARGET_ROLE_NOT_ALLOWED`.
- §5 Every role write is held to the subset rule (`access.CanActOn`): a caller acts only on a role whose permissions, `learning.take_tests` apart, are their own, grants or revokes only keys they hold, and never changes their own role's permissions, except the Admin role's `learning.take_tests` cell. A matrix save is one transaction and refused whole. It and `updateRole` make every cached principal on the machine that served them be read again on its next request.
- §13.5 The app role may now insert roles, update the columns a role command changes and add and remove grants (`00129_grant_role_writes.sql`). It still cannot delete a role or write `app.permissions`.

**Changes since v0.108**

v0.11.7, a hotfix after the owner's report (2026-10-14): dragging a question into a group and out again left a second bank question each time (`docs/plan/74e-e1-group-move.md`, "After release").

- §7.3 A member copy remembers the bank question it was made from. Dragged out of its group with its content as it was copied, it goes back as that original: the copy is removed, the original takes its place in the section at the drop position, and no bank question is added. A copy that was edited since stays a new bank question, as before, and the builder says it was saved as a new question because it had been edited. A copy whose original is deleted, is another teacher's or is already in the test leaves as a bank question without a word. A move between groups or within one keeps the memory.
- §15 `POST /teacher/tests/:id/questions/move` answers `restored` and `editedCopy` beside `copied`. No new operation or permission. One nullable column, `questions.copied_from_id`, and its index (`docs/plan/20-data-model.md` D-50, §48).

**Changes since v0.107**

E1: v0.11.6, after a teacher's escalation the owner decided on 2026-10-14 that a question can be dragged into a shared-context group in the builder and out of one (`docs/plan/74e-e1-group-move.md`). It reverses D4's default, which kept a bank question out of a shared group (`docs/plan/74d-d4-deck-update.md` Q8, DG-186).

- §7.3 A standalone question dragged into a section's group becomes a member copy and the bank question stays. A member dragged out keeps its id and becomes a bank question and a unit of its section. A member moves between a test's groups, or within its own, in place. A member that a material gap is bound to does not leave its group. The test revision and the revision of each touched group guard the move. The next publish freezes the new structure, and a version diff against the draft reads a copy into a group as one question removed and one added.
- §8 The builder's outline: a grip on each member row, the group's header as a drop target, and a menu and a keyboard path for every move.
- §15 `POST /teacher/tests/:id/questions/move`, the only operation added. No migration, no new permission, and no change to a published version, a deal or an attempt.

**Changes since v0.106**

v0.11.4, a hotfix after the owner's report (2026-10-14).

- §8, §15 "AI check" reads the members of a test's shared-context groups as well as its standalone questions, in the builder's order and each with its group's passage; a question's number is the builder's, one per question counting across the sections. It answered "There is nothing for the assistant to read yet." for a test whose questions were all members of a group, because it read only the standalone questions of a section.
- §8, §15 "Make a variant" still covers standalone questions only. A variant duplicates bank questions and writes each change to the duplicate, and a group's members cannot be duplicated that way: the members are copied as they are, and a test whose questions are all members answers the same message.

**Changes since v0.105**

v0.11.3, a hotfix after a teacher's report (2026-10-14).

- §12 The builder's Generate adds its questions where their source belongs: from a passage a shared-context group holds, to that group as new members; from pasted text, a topic or nothing, to the section, as before. The builder used to append every generated question to the section, so questions written from a group's passage landed outside the group that shows it.

**Changes since v0.104**

D4, T-D4.18: the release review of listening (#602).

- §11.3 Leaving the test page forgets every remembered place, as a reload does, so coming back to the same attempt or starting another spends a play. The text said "cleared with the attempt"; the player restored a place after the page had cleared it, and a new attempt in the same tab could resume one for free.

**Changes since v0.103**

D4, T-D4.12: the engine and the teacher's AI surfaces described as D4 built them. No behaviour changed in this version; the text caught up with T-D4.4 to T-D4.10a, T-D4.22 and T-D4.36 to T-D4.38.

- §9 The engine shows one question at a time, except a run of a cloze's gaps, which is one screen with its own line, chips and gap card, stepped over as a whole; every gap keeps its number, square, flag and points (T-D4.10a). A section's title and instructions stand above each of its questions, not only the first (T-D4.5). The stimulus pane's kicker and heading and the question pane's surface and widths are stated. "Navigation" replaces "Footer": the sections rail from 900px, or 1180px beside shared content, with the key hint in the footer; the grid button and the question sheet everywhere else, a bottom sheet below 768 and a 560px card from it; three square states; no strip (T-D4.4). The arrows move by screen, and Esc also closes the drawer.
- §12 "One breakpoint, 768px" is the shell's and the engine's chrome; a new bullet records the engine's other thresholds (420, 900, 1180, the drawer's 88% up to 420px, and the 1024px sizes) and their one width source. The touch-target bullet names the rail's 34px squares, the drawer's handle and close button and the sheet's 44px and 42px squares in place of the strip; the engine's question column is 720px alone. The recording card's lime track is the audio player rule's one exception.
- §10.4 The assignment's Flagged count leaves out voided attempts, as T-D4.22 built it.
- §8 The grading card and Assignment detail › Questions draw a line, not nothing, while "Let AI read student answers" is off; Settings › AI assistant says the date the allowance resets and shows a waiting request. No "Availability: D4" marker is left in §8 or §12.

**Changes since v0.102**

D4, T-D4.21b: Grading when nothing is left to mark, and papers ready to finish.

- §8 The grading page's empty states and its "Sẵn sàng hoàn tất" list: "Đã chấm xong tất cả" only after a complete check of the papers, each paper's check read fresh before Finish, the next paper's check after a Finish, and the list folded below the panes while answers wait. Finish stays explicit.

**Changes since v0.101**

D4, T-D4.16: the teacher's three previews draw the take-test engine in their frames.

- §7.3 and §12 The builder's preview, Test detail and the import's "Preview and create" hold the take-test engine, answerable, with what is chosen kept only while the preview is open and nothing saved, recorded for integrity or counted as a play. The header has the "Student view" chip in Submit's place and no leave button; the timer's place holds a pill reading "––:––", named "The time limit is set when the test is assigned"; the save line reads "Answers here are not saved."; Finish says "This is a preview. Students submit here." (DG-188).
- §12 The engine's 1024px sizes and its 44px floor follow a preview frame's width, so the phone frame on a wide window draws a phone's controls.

**Changes since v0.100**

The bank clean-up is a bounded pass, not one call (T-D4.32 follow-up 1).

- §15 One `cleanUpQuestionBank` request reserves its 200 credits once, makes at most five calls to the provider at the same time inside 45 seconds, and ends with one settlement of what the calls were billed. Tags are suggested for the first 200 questions without any, 50 to a call; duplicates are looked for by comparing every pair of the questions read by their words, and one call judges the closest pairs, cut to the best that fit 100,000 bytes, at most 20 returned. The 300,000-character cap and the first-60 cap are gone; `truncated` also covers a part of the pass that failed or ran out of time, a duplicates call cut to fit its size, and a search for look-alikes that stopped at its work limit. The shapes of the request and the response did not change.

**Changes since v0.99**

D4 follow-ups: the grading leave guard counts a filled score, and the phone's shared content is described as T-D4.7 built it.

- §12 On the grading card, leaving the page with a score filled from the suggestion and not yet saved asks first ("Rời trang khi điểm chưa lưu?"), as a typed comment already did.
- §7.3, §9 and §12 Below 768px a group's shared content is a drawer from the left over the question, opened from a handle at the screen's edge or by a swipe, not the "Ngữ liệu | Câu n" switcher T-D4.7 removed. The Panes bullet says how it opens, closes, nudges the first time and keeps its place; nothing it describes changed in the product.

**Changes since v0.98**

D4, T-D4.40: the AI assistant (D19 to D22, decided by Thuong on 2026-10-10). The server side is built (T-D4.30 to T-D4.33); the web surfaces are T-D4.34 to T-D4.39, and all have shipped: T-D4.34, the panel and the builder, bank and Test detail entry points; T-D4.35, the question editor's "Write with AI" and "Suggest wrong options"; T-D4.36, the grading card's "Suggest a score"; T-D4.37, Explain results; T-D4.38, Settings › AI assistant; and T-D4.39, the import card.

- §1.3 The assistant suggests and never decides, and its credits are a budget, not a payment.
- §2 The provider is Anthropic's API, called through `platform/anthropic` behind the ai module's `Completer` port. Two tiers, their models and their prices are configuration, and no model identifier is committed. With `ANTHROPIC_API_KEY` unset the assistant is off.
- §8 The teacher's AI surfaces, Settings › AI assistant, and the Admin "AI credits" page (Availability: R5; until then `cmd/maintenance ai-credits`).
- §12 The panels' rules: the acceptance rule, the disabled-with-reason rule, the credit note and the failure states.
- §13.10 The six AI tables, the two constraints that make the pool a hard stop, and the append-only `ai_usage_ledger`.
- §15 Sixteen operations under `/teacher/ai/*` and `/admin/ai/*`, `PublicStatus.ai`, `UserPreferences.ai`, `readWithAi` on the import, three notification kinds (twelve in all), the AI error codes and the rate limits.
- §16 "Read with AI": a scanned PDF is read by the assistant when the teacher's import asks for it, and is refused as before when it does not. The "no OCR" and "text layer only" sentences say so.
- §17 Decision 4: cloud AI processing is on for the assistant's features (D19), with per-feature minimisation (D20) and a pool-constrained budget (D21).
- "Changes since v0.7" and "Changes since v0.41": the sentences that kept cloud processing a candidate and ruled out OCR are marked superseded for these features.

**Changes since v0.97**

D4, T-D4.9: a recording that cannot be paused (DG-183).

- §11.3 With `allowPause` false the card's button is inert while the recording plays (`aria-disabled`, 45% opacity) and the status says the recording can't be paused, before the first play and while it plays. A pause from elsewhere (a headset button, another recording, leaving the question or the group, a lock) still happens, reads "Paused", and Resume continues it for no play; nothing new is recorded.

**Changes since v0.96**

D4, T-D4.8: the recording card in the stimulus pane, and what spends a play (DG-193).

- §9 A shared recording's card stands at its audio node in the stimulus pane, the drawer below 768px, and no longer in the question pane; with several recordings each card is headed by its node's label.
- §11.3 The engine's player is the deck's recording card: a 52px button (Play, Pause, Resume, Play again), the track, the time and a play counter, and a status line. A play in progress keeps its place for the tab's life when its question or group is left, and starting one player pauses the others.
- §11.4 A play is spent when playback starts with no play in progress; a resume spends none and sends no `audio_play`. A place that cannot be put back counts as a play.

**Changes since v0.95**

The import workspace of the approved page-width design (DG-207, Decided).

- §8 Upload and setup and Processing are one page per import: the Sources pane beside the status pane, stacked below 960px of content. `/teacher/imports/new` is the draft and starting it replaces the address with `/teacher/imports/:id`, which stays the same through every state; a committing or committed import redirects to its confirm page. Pasted text is read as it is typed in the status pane, listing only what needs checking.

**Changes since v0.94**

The page frame's follow-up (DG-205): the lists' columns follow the approved sketch, and Settings caps its columns.

- §12 The lists follow the design's sketch: Assignments shows Assigned to from 1240px of content, Submitted from 760 and a To grade column from 900; the Imports history adds Source from 1240. Settings' fields take at most three columns.

**Changes since v0.93**

R4, F-68 web part: the media library says what becomes of a replaced or deleted file, before and after.

- §11.2 The Replace and Delete dialogs say, before the action, that the old file is kept for the published tests that use it (each test counted once) and deleted seven days after none does, or, when none does, deleted for good seven days later and not restorable, and offer the file's download. The toast after it names the date from `previous.deleteAt` or `deleteAt`, or that the file is kept for the tests that hold it. The Storage card counts `heldBytes` in what is used and lists the held and the pending files with the earliest date. A replacement refused for room while published tests hold the file says the library needs room for both files, and a 429 says how long to wait. No bell, no restore (DG-206).

**Changes since v0.92**

R4, F-68 hardening: the sweep can be switched off, and its last step survives the end of its time.

- §11.2 `MEDIA_RETENTION_SWEEP` (`on`, the default, or `off`) stops the media sweep on the machines that read it. Once the sweep has deleted an object it finishes the mark, the audit row and the commit under their own time, and logs the file at ERROR if it still cannot. A bank question's write that names a removed file in that instant answers as a missing file, not a 500.

**Changes since v0.91**

The design team's page-width design, approved by the owner on 2026-10-12 (DG-205, Decided). Numbered v0.92 to stay clear of v0.90, the deck update on `work/deck-d4`.

- §8, §12 Every route in `TeacherLayout` renders in one page frame: the full content width up to `--page-max` (1600px), centred beyond it at every width, 2560 included, inside gutters of 14, 28 and 40px. A route's handle no longer declares a width (it was 1320 by default, 1080, 860, 720 or full); the import review alone keeps `width: "full"`. Containers fill the frame; a field is at most 480px wide and prose at most 72ch. The student app's columns are unchanged.

**Changes since v0.90**

R4, F-68 (v0.90 is the deck update on `work/deck-d4`): a replaced or deleted media file is kept for the tests that use it, then removed.

- §11.1 The quota counts the library and the replaced or deleted files that something still uses, so a replacement that published versions hold needs room for both files. `replaceMedia` answers `previous` (the old file, the versions that hold it, a URL to download it, and when its object goes), declares its 429, and `deleteMedia` answers 200 with the same shape where it answered 204. `usage` gains `heldBytes`, `pendingBytes` and `pendingDeleteAt`.
- §11.2 A file that left the library stays in storage until seven days after nothing names it, then the API's sweep deletes its object and marks the row; nothing restores it and nothing notifies the teacher.
- §13.3 `media_assets` gains `unreferenced_at` and `purged_at`; a trigger on the six tables that name a file refuses a purged one and stops the wait of a retired one (migrations 00111 to 00113, `docs/plan/20-data-model.md` §45).
- §15 The media lines.

**Changes since v0.89**

D4, T-D4.13 (DG-183): a teacher can stop students pausing a recording.

- §7 `AudioPolicy` gains `allowPause: boolean`, default true. A student receives `StudentAudioPolicy`, the same four fields all present, for a question's audio and for a group's recording.
- §11.1 The audio controls are four: `maxPlays`, `allowSeek`, `showTranscriptAfterSubmit` and `allowPause` ("Students can pause", default **true**), on a question's audio and on a group's recording. A create that leaves it out stores true. An update that leaves it out keeps the stored value, of the same question by its id, of a group's member question by its id in the bundle and of a recording by its id, so a tab loaded before the release or a recovery draft from the device cannot switch pausing back on; an audio policy or a recording that is new in the request is a create. Publish, restore as a draft, duplicate and group copy carry the value, and a version freezes it.
- §13 `questions.audio_allow_pause` and `test_version_questions.audio_allow_pause` are nullable and read as true when NULL; `group_recordings.allow_pause` and `test_version_group_recordings.allow_pause` are `boolean NOT NULL DEFAULT true` (migrations 00114 to 00117).
- §15 `createQuestion`, `updateQuestion`, `createQuestionGroup` and `updateQuestionGroup` accept `allowPause` and answer with it; the student's paper, the preview and the result send `StudentAudioPolicy`.

**Changes since v0.88**

R4, M1 of the v0.10.0 release review: a device's choices from v0.9.0 move to the account once.

- §5.4 After sign-in or a session restore, a theme, language or "Larger text in tests" the account does not hold, and that this browser stores as a choice made there (`quizzivy.theme`, `quizzivy.locale`, `quizzivy.testText`), is applied and saved to the account once, through the same writes Settings uses. A value the account holds always wins; nothing is written when no valid choice is stored, nor for a value the app copied there from an account; a failed save keeps the device's choice on screen, says nothing, and is tried again at the next sign-in.

**Changes since v0.87**

R4, F-67: a replaced legacy join code keeps its use count.

- §6.1 The sealed code `jobs.RotateLegacyJoinCodes` issues keeps the old code's use count with its expiry, use cap and creator, and no longer starts at 0 uses. A class whose legacy code had reached its cap gets a code that is used up (`JOIN_CODE_EXHAUSTED` on the next join), not one that admits `max_uses` more students; its teacher issues a new code when they want to admit more. A code below its cap keeps what was left of it, and a stored NULL cap stays NULL with its count carried.

**Changes since v0.86**

R4, F-66: a sign-out finishes when the client has gone.

- §5.4 `POST /auth/logout` revokes the refresh family under its own ten-second budget, which the end of the request does not cancel, so a tab closed during sign-out still ends the session. A refresh keeps running on the request's own context; the window it leaves (a rotation committed whose answer never arrives) is recorded under R-06 in `docs/plan/30-risks.md`.

**Changes since v0.85**

R4, the sections brought level with the release (T-R4.50, the Tech Lead's half). No behaviour changes: each line below is a body section that still described the console before R4.

- §2, §3, §5.4 The teacher's tree is `/teacher/*` in `TeacherLayout`; `TeacherWorkspace` holds it, `StudentArea` and `homePathFor` send staff to `/teacher`, and a `/admin/*` address redirects to the same path under `/teacher` until R5 builds the Admin console at `/admin`. `AdminLayout` is gone.
- §7.1 Prompts and explanations are written in the rich editor for every teacher; the opt-in `VITE_RICH_QUESTION_EDITOR` is gone. Options stay plain inputs unless `VITE_RICH_OPTION_EDITOR` is on.
- §8 The teacher's routes and screens as R4 built them, with the shell: the sidebar, its stored state, the 768px drawer and the content-width thresholds.
- §9, §12 The teacher console follows the theme; nothing forces light any more.
- §10.4 The teacher's integrity view (D8): the roster's focus-loss count and flagged rows, the attempt sheet's compact timeline, the attempt page's Integrity tab, one danger tone for "Flagged".

**Changes since v0.84**

R4, the sections brought level with the release (T-R4.50, server half). No behaviour changes: each line below is a body section that still described an earlier state or never described an R4 change.

- §6.1, §6.4, §6.5, §13.3, §13.5 The legacy rotation is done, not planned. The text now says what `jobs.RotateLegacyJoinCodes` does at start-up (per-class transaction under advisory lock `(73819, 41)`, the two-minute budget, what the new code keeps, the audit, the teacher's notice) and what stays until R5 (the scheme-1 lookup for a revoked or expired row). The class screen shows a code in full, at `/teacher/classes/:id`.
- §11.1 A question's media is one audio file or one image; an image carries `mediaAlt` (1 to 1000 characters), which belongs to the question, is frozen at publish, reaches students beside `media`, and is refused for any other media. Replacing a file with one of the other kind is `415 MEDIA_KIND_MISMATCH`.
- §13.3 The media library's columns and the size limit by kind (D-29), the question and frozen level and skill (D-31), `test_versions.change_note` (D-32), the assignment's review options and note (D-34, D-35), the rule that an attempt in progress follows its window and an attempt starts under the window as it stands when it commits, the notification tables (D-28) and the import draft counters and text source columns (D-40, D-41). §13.7 records a data-only migration with a Down that does nothing (00103, D-38).
- §15 The test versions' operations and fields (`changeNote`, `testUpdatedAt`, `assignmentCount`, the diff), `MonitorRow.extendedTo`, the Word import operations including `pasteImportSource`, and the notifications' kinds, switches and producers. The teacher's lists are offset-paginated by `page` and `limit`; the `cursor` parameter the tests, questions and media lines named does not exist, and only `GET /me/notifications` is keyset.
- §16 The pasted-text source as built: the paste mode of the upload page, plain text only with `TEXT_MARKS_UNAVAILABLE`, the 100,000 code-point and 20,000-line bounds, the conventions `rules-v3` reads for text sources, and the one-line answer key read in Word and PDF files too (#233). Recognition is `rules-v3`, no longer `rules-v2`.

**Changes since v0.83**

R4, an archived class takes no new join code (F-64):

- §6.4, §15 `rotateJoinCode` (`POST /teacher/classes/:id/join-code`) on an archived class is
  `409 CLASS_ARCHIVED`, a new `ErrorCode`, and writes nothing: the class's current code, if it has
  one, stays as it was. Nobody can join an archived class (its code answers as an unknown code, in
  the preview and in both joins), so a code issued for one was a dead secret that read as live. The
  class is found for the caller first and checked for archiving after, inside the transaction that
  holds the class row, so another teacher's archived class still answers `404` as a missing class
  does, and an archive racing a rotation is ordered with it by the row lock: one that committed
  first is seen and refused, one that arrives later waits for the rotation. An Admin with
  `scope.all` reaches the class and is refused with `409`. `revokeJoinCode` stays allowed on an
  archived class, and restoring the class (`archived: false`) lets it rotate again. The web app
  already hid the action (PR #509); the server now refuses it too.

**Changes since v0.82**

R4, a NUL character in a request (F-24):

- §15 The character U+0000 is refused with `400 VALIDATION_FAILED`, wherever the request carries
  text: a string or an object member name anywhere in a JSON body (the JSON a `text/plain`
  beacon carries too), a query parameter's name or value, and a decoded path segment. PostgreSQL
  stores it in no `text` or `jsonb` value, so every free-text field, every free-form `meta` and every
  `q` or `tag` search answered `500` before. The refusal names the field by its dotted path
  (`options.1.text`, `tags.0`; for a member name, the object that holds it) or the parameter, in
  the message, as the validator's other refusals do, and in `details` as `{ field: message }`. It is decided in the one
  place the contract is enforced: it covers the nine open operations too, after the rate limiter
  has counted the request. A backslash followed by the characters `u0000` (JSON `\\u0000`) is ordinary
  text and is not refused. The file name of a media upload or replacement is refused the same way
  when it holds U+0000 (`400 VALIDATION_FAILED` naming `filename`; an RFC 2231 `filename*` could
  carry one past the multipart parser, and `POST /teacher/media` answered `500`). A Word import's
  file name with any control character was already `415 IMPORT_SOURCE_UNSUPPORTED`.

**Changes since v0.81**

R4, `closeNow` before the window has opened (F-63):

- §15 `closeNow` closes an assignment now, so it needs a window that has opened. A request with
  `closeNow` and `window.opensAt` still ahead of now is `400 VALIDATION_FAILED` naming `closeNow`,
  whatever the assignment's status: a scheduled assignment closed now, an open one given an
  opening ahead, a closed one given an opening ahead. All three answered `500` before, because the
  schema requires `closed_at >= opens_at`. On a closed assignment this takes the case "closed now
  beside a window that opens later" away from `409 ASSIGNMENT_CLOSED` (v0.81); a window that opens
  after an early close the assignment already keeps, with no `closeNow`, stays `409`.
- §15 `closeNow` is a field of a PATCH. A `POST /teacher/assignments` ignores it, as the contract
  said ("PATCH only"): a new assignment is never closed, and a create with `closeNow` and an
  opening ahead answered `500` before.

**Changes since v0.80**

R4, a closed assignment is opened again only by `reopenAssignment` (DG-160):

- §15 `PATCH /teacher/assignments/:id` no longer reopens a closed assignment. Judged at the request's
  clock, an assignment that is closed, by its window or by an early close, stays closed: a window
  that would make it `scheduled` or `open` again, or start after the early close it keeps, answers
  `409 ASSIGNMENT_CLOSED`, the code `extendAssignment` and the student overrides already answer
  for a closed assignment. Reopening it goes through `POST /teacher/assignments/:id/reopen`, which
  requires a reason and writes `assignment.reopened` to the audit log; a save that moved the close
  made it open again through `assignment.updated`, with neither. Nothing else about a closed
  assignment changes: the stored window, an earlier close, `closeNow` and every other field are
  saved as before, and so is a later close on an assignment closed early, which stays closed
  because a save never lifts an early close.

**Changes since v0.79**

R4, a bell counts, lists and clears only its own console's notifications (DG-170):

- §15 `getTeacherSummary` (`GET /teacher/summary`), `getMySummary` (`GET /me/summary`) and
  `listNotifications` take an optional `audience` query parameter, and `markNotificationsRead`
  an optional `audience` in its body, each `teacher` or `student`. With one, the unread count,
  the page and the marking cover only the kinds that console shows. The teacher's are
  `attempt.submitted`, `attempt.flagged`, `assignment.closing`, `class.joined` and
  `join_codes.rotated`; the student's are `assignment.opened`, `assignment.due_soon`,
  `assignment.extended` and `result.ready`. Every kind belongs to one of the two. The audience
  narrows a marking by ids as it narrows "mark all": an id of the other console's kind is
  skipped, as another user's id is, and answers 204 all the same. Without an audience every
  kind is covered, as before. Any other value is 400 `VALIDATION_FAILED`.
- §15 An account that holds both consoles (an Admin with "Take tests") has two bells that neither
  light nor clear each other. Both bells passing `audience` is the web's part of DG-170.

**Changes since v0.78**

R4, ownership is validated (T-R4.49, T-R3.2):

- §13.3 Migration 00106 validated the `NOT NULL` of `classes.teacher_id` and of `owner_id` on
  `tests`, `questions`, `question_groups` and `media_assets`, and dropped the triggers and
  functions that filled an owner for the v0.7.0 binary's inserts. An insert that omits an owner now
  fails with `23502`. The previous binary (v0.9.0) names the owner in every insert.

**Changes since v0.77**

R4, the legacy role goes (T-R4.49, T-R3.1):

- §5.2 An access token carries the user, its times and the session epoch, and no role. A token
  minted before this release still names one, and it is ignored.
- §7 `Role`, `User.role` and `CurrentUser.role` are gone. A caller's authority was already
  its permissions; a client that wants to know whether the signed-in user only learns reads
  `workspaces`.
- §13.2, §13.3 `app.user_role`, `users.role`, its index and the trigger `users_sync_legacy_role`
  are dropped by 00105; `users.role_id` is `NOT NULL`, validated. The previous binary (v0.9.0)
  never names the column.

**Changes since v0.76**

R4, the bulk password reset and the student filters (T-R4.20):

- §5.4 and §15 `POST /teacher/students/reset-passwords` (`people.students.reset_password`)
  takes `studentIds` (1 to 40, no repeat) and answers `200` with `items` (`studentId`,
  `fullName`, `email`, `temporaryPassword`) for the students it reset and `failed` (`studentId`,
  `code`) for the others, each list in the order asked, with `Cache-Control: no-store`. A
  failure is `NOT_FOUND` (another teacher's student, an account that is not a student, a
  disabled student and a missing id are one answer), `FORBIDDEN` (the subset rule),
  `STUDENT_SHARED` (someone else also reaches the student; a holder of `people.users.manage`
  resets them) or `INTERNAL` (a fault, logged with the student's id and the cause, never the
  password). The call answers `200` even when every student failed, and never fails once a
  student is reset, because its answer is the only place the passwords exist; a lost answer
  is recovered by listing `mustChangePassword=true` and resetting again.
- §5.4 Each student is reset by the single reset's own write, after the same guards in the same
  order, in a transaction of its own: the hash, `must_change_password`, the session epoch, every
  refresh family and the audit row `student.password_reset` change together, and the student's
  cached principal is forgotten after the commit. Two students are worked on at a time, inside
  `MAX_CONCURRENT_PASSWORD_HASHES`. A cancelled request starts no further student and keeps
  those already reset.
- §5.4 A temporary password, from `createStudent`, `resetStudentPassword` and
  `resetStudentsPasswords` alike, is three words and a two-digit number, `word-word-word-NN`,
  about 24.4 bits (it was `word-word-NN`, 18.45), so that guessing it at §6.5's sign-in limit
  (20 an hour per email) takes about 560,000 hours on average, where it took about 8,900. Nothing
  else about it changes.
- §6.5 `resetStudentsPasswords` 2/min and 10/h per signed-in user.
- §15 `GET /teacher/students` takes `classId` more than once (a student in any of those
  classes, counted once; a single value works as before) and `mustChangePassword`; its facets
  count the same students as its list.

**Changes since v0.75**

R4, the class schedule and room, the end-of-day expiry, class averages and the codes in the class
list (T-R4.18):

- §6.1 A code issued with `expiresInDays` n works through 23:59:59 on the day n days after today
  in the issuer's calendar zone: the profile preference, or `Asia/Ho_Chi_Minh` when there is none
  or it cannot be read. The day is counted on the wall clock of that zone, so a daylight-saving
  change cannot move it, and a code made at any hour of a day expires at the end of the same day.
  An Admin who rotates a teacher's class is the issuer, and the Admin's zone counts.
- §6.4, §15 `listClasses` rows are `ClassListItem`s, which repeat `Class` with `joinCode` read in
  full. `withCodes=true` opens each row's code, and only for a holder of `teaching.classes.write`;
  a code opens in the list exactly when `getJoinCode` would open it for the same caller and class
  (through the same read, so a legacy code, or one sealed under a key the server no longer holds,
  shows `null`, and a code that does not authenticate shows `null` and is logged by class without
  the code). A caller without the permission who asks gets `null` codes and no error. Every
  `listClasses` answer is `Cache-Control: no-store`, and the operation is limited to 60 a minute
  and 600 an hour per user, as `getJoinCode` is. Pickers do not ask for codes.
- §7, §15 `Class` gains `scheduleLabel` (1 to 120 characters) and `room` (1 to 60), both nullable,
  and on teacher responses `averageScore`, an `AttemptScore`: the sum of earned over the sum of
  total points of the best graded attempt (the highest share, the later attempt on a tie) of each
  live member on each assignment that targets the class, with the unmarked manual answers of those
  attempts as `pendingManual`. A student who left, a disabled student, a voided attempt and an
  attempt not yet graded count for nothing; `null` when nothing is graded. `createClass` and
  `updateClass` take `scheduleLabel` and `room`: trimmed, blank is none, and on update an omitted
  field keeps the value while `null` or blank clears it.
- §9, §15 A student's class (`GET /app/classes`) gains `scheduleLabel`, `room` and
  `teacherAvatarUrl`, each `null` when there is nothing to show, and still no count, member list,
  code or average. The class a student joins carries the schedule and room but never an average
  or a code.
- §13.3 `classes.schedule_label` and `classes.room`, nullable, migration 00104.

**Changes since v0.74**

Composed text on the server (F-37, FONT-1):

- §12 Typography: the server stores the text a person types composed to NFC, so "stored as
  typed" (v0.67) no longer holds for it. A write composes the plain fields (titles,
  descriptions, names, notes, reasons, tags, accepted answers, transcripts, alt text, file
  names), the `text`, `label` and `alt` of a prose document and the Markdown of a legacy one,
  and the text a student types in a text or fill-in answer when it is saved. A document and
  the plain text kept beside it are composed together, so the text still equals the
  document's projection. Emails, passwords, tokens, join codes, ids, links and the published
  versions are never composed. NFC shortens Vietnamese, but a few scripts grow when composed:
  a value that composing leaves over its limit is answered as any other invalid value, 400
  `VALIDATION_FAILED` naming the field (an import commit answers a refused question the same
  way; a group answers 422 `group_content`), never as a server error. A name or a file name
  that comes from a provider or a file is kept as it came, and a student's save is never
  refused over it.
- §13 Migration 00103 composes the rows written before, in the same plain fields, without moving
  `updated_at`. It leaves the published versions, a student's saved answers and every prose
  document, with the plain text beside it, as they are: those are composed when next saved.
  A view still composes what it draws, because those rows can stay decomposed.

**Changes since v0.73**

R4, the starter questions (T-R4.31b, T-R4.34):

- §7.1 The builder's and the group composer's starter prompt is created as rich text, so a new
  question in either opens in Rich text; while its text is still the starter it opens empty
  under the placeholder, and the starter stays stored until the first edit.

**Changes since v0.72**

R4, the next version number (F-46):

- §7 and §15 `Test` gains a required `nextVersion`: the number the next publish takes, one more
  than the highest version ever published, deleted or not (`tests.last_published_version + 1`),
  so it is 1 for a test never published. Deleting the newest version, which is allowed while
  it is not the default, does not lower it. The publish command and every read of a `Test`
  (`getTest`, `listTests`, the answer of a write) take it from the same function, so they
  cannot disagree. The test detail's draft banner and publish dialog name it in place of the
  highest listed version + 1, which named 3 where the server assigned 4 (VER-30).

**Changes since v0.71**

R4, profile photo (T-R4.8):

- §15 `PUT /me/avatar` (`self`, multipart, a file of at most 2 MiB in a request of at most
  2 MiB + 16 KiB) stores the caller's photo as a 256×256 PNG made from the centred square of a
  PNG or JPEG whose sides are each 200 to 2048 pixels, EXIF orientation applied and every
  other metadatum dropped, and answers `CurrentUser`; `DELETE /me/avatar` clears it and
  answers the same. `CurrentUser.avatarUrl` is a presigned GET valid 24 hours. A new
  `ErrorCode`, `IMAGE_DIMENSIONS` (415), answers sides out of range, an image that would need
  more memory to decode than the server allows (48 MiB) and a JPEG of more than 32 scans;
  `MEDIA_TOO_LARGE`, `MEDIA_TYPE_UNSUPPORTED` and `MEDIA_UNREADABLE` keep their meaning.
  `PUT` is limited to 10 an hour per user. `User` (someone else) carries no photo yet.

**Changes since v0.70**

R4, signed-in devices (T-R4.9):

- §5.2 Signed-in devices. `GET /auth/sessions` lists the caller's live sessions, and `DELETE
  /auth/sessions/{familyId}` and `POST /auth/sessions/revoke-others` end one or all but the
  caller's. The location shown is where the device last signed in or last refreshed, not where
  it first signed in. Both writes move the caller's session epoch, so the caller's own next
  request answers 401 and recovers through the single-flight refresh. A request whose refresh
  cookie names no live session of the caller is answered 401 by both writes; the client's normal
  refresh then fails and the sign-in overlay shows.
- §6.5 `DELETE /auth/sessions/{familyId}` 10/min and 60/h and `POST /auth/sessions/revoke-others`
  5/min and 30/h, per signed-in user.
- §13.5 `refresh_tokens.geo_label` (1 to 80 characters, nullable), migration 00102.
- §15 The three operations, `Session`, and the error code `SESSION_IS_CURRENT`.

**Changes since v0.69**

R4, the skills of a published version (F-43):

- §15 `TestVersion` (`publishTest`, `listTestVersions`) gains a required `skills`: the sorted
  distinct skills of the questions the version froze, a group's members included, from the same
  six values as `Test.skills`. A version whose questions carry none, which is every version
  published before T-R4.15, answers `[]`. It is read from the version's own rows, so editing the
  draft or the bank after the publish does not change it.

**Changes since v0.68**

R4, notification producers and due-time items (T-R4.10b):

- §15 Who is told, and when. A notification is written after the change that caused it has
  committed, under a budget of its own that the end of the request does not cancel; a
  failure is logged and never fails the request. The teachers are the assignment's creator
  and the teachers of a target class the student belongs to (`visibility.PaperReaders`, the
  rule that lets a teacher see a paper, read the other way), each once, so a teacher of
  another class is told nothing about this student. `attempt.submitted` merges the papers
  handed in on one assignment into one notification per fifteen minutes, a timed-out paper
  being a hand-in; `attempt.flagged` is written once, when the integrity policy flags a
  paper, and never for a teacher's own flag or under `warn`; `class.joined`, which has no
  switch, on a join by code. The student is told `assignment.extended` when `notify` is
  true on an extension (everyone whose close it moved) or on an override that moves a
  close (those it gave time past the assignment's own close), and `result.ready` when a
  teacher finishes grading and the review policy shows the score at that moment. A
  `result.ready` notification carries the title and the attempt, never a score.
- §15 Due-time items. `GET /me/summary` and `GET /me/notifications` first write what the
  passing of time has earned the caller, at most once in five minutes per caller in the
  serving process; there is no timer and no job. A moment older than seven days is not made
  up, a row the caller already holds is left as it is, read or not, and a switch that is
  off writes nothing. For a student: `assignment.opened` (when the assignment opened or was
  published, whichever is later), `assignment.due_soon` a day and an hour before the
  student's own close, override and early close included, while they have handed nothing in
  and have an attempt left and the test was available at that moment, the day's only while
  the hour's is not yet due (a student who first reads inside the last hour gets the
  hour's alone), and `result.ready`
  at the close of an `after_close` release for a paper with nothing waiting for a mark. For
  a teacher: `assignment.closing` an hour before the assignment's close, with the number of
  students they reach who have not handed in and whose own close is the assignment's. A
  close that moves earns its reminders again.
- §9 The Test intro says "We will notify you when it opens." under a test that has not
  opened for a student whose "Test due soon" switch is on, and "You can start once it
  opens." otherwise.

**Changes since v0.67**

R4, duplicate, item analysis and the results export (T-R4.13):

- §15 `POST /teacher/assignments/:id/duplicate` (`teaching.assignments.write`) takes
  `{classIds}` and creates a draft that pins the same version and keeps the original's
  time limit, attempts, shuffle flags, review options, integrity policy and note to
  students. Its window opens now and lasts as long as the original's; it is assigned to
  the classes named and to no student by name, and is owned by the caller. An original the
  caller does not reach answers `404`, one whose version the caller may not assign answers
  `409 TEST_NOT_PUBLISHED`, and a class the caller does not teach answers `400` on
  `classIds`.
- §15 `GET /teacher/assignments/:id/item-analysis` (`teaching.grading`) answers
  `{handedIn, items}`: for each question of the pinned version, hardest first, how many
  papers answered it and the share that earned its full points. The papers are, for each
  student the caller reaches, the latest attempt that is handed in and not voided. A
  question left unanswered counts against it, a manually marked answer not yet marked
  counts for neither side, and a question no paper has a mark for has a null rate and
  comes last.
- §15 `GET /teacher/assignments/results.csv?ids=` (`teaching.grading`, 1 to 50 unique ids)
  downloads a UTF-8 CSV with a byte-order mark: a row for every student of each assignment
  whom the caller reaches, a student who has not started included, with the attempt the
  monitor shows for them. Its times are in the caller's own time zone, and its headers and
  status words follow `Accept-Language`. A cell a spreadsheet would run as a formula is
  written with a leading `'`. An assignment the caller does not reach answers `404` for the
  whole request, and more than 20000 rows answer `422 VALIDATION_FAILED`. A caller may
  download 10 files a minute and 60 an hour.
- §15 `GET /teacher/assignments` takes `q` (the test's title or the name of a target class
  the caller reaches, ignoring accents) and `classId` more than once (any of the classes),
  and its facets follow both. Every assignment carries `questionCount`.

**Changes since v0.66**

Characters the brand font draws (FONT-1, DG-141):

- §12 Typography: the italic faces are loaded for every upright weight. Every character
  the UI draws is in the loaded font faces; a symbol the font lacks (arrows, ⌘, ⏎, ⇧, ≤,
  ≥) is an icon, or a character it has, such as the en dash for a range.
  `font-coverage.test.ts` holds the source and the locales to it.
- §12 Typography: content is shown in NFC. Be Vietnam Pro's subsets lack the combining
  circumflex, breve and horn, so decomposed Vietnamese would draw in a fallback font. The
  rich content editor composes what it writes; a plain field, the Markdown source and a
  student's answer are stored as typed and composed where they are drawn (F-37 proposes
  composing stored text on the server).

**Changes since v0.65**

R4, extensions and per-student overrides, second half (T-R4.12b):

- §13 An attempt in progress follows a change to its window. `POST
  /teacher/assignments/:id/extend`, `PATCH /teacher/assignments/:id` (a later close),
  `POST /teacher/assignments/:id/reopen` and `PUT
  /teacher/assignments/:id/student-overrides` set the deadline of each attempt in
  progress to the earlier of its start plus its student's time limit and its student's
  close, in the transaction that changed the window, and only where that is later than
  the deadline it has. A deadline never moves earlier: removing an override (`DELETE`),
  setting a shorter time limit or a shorter close, and closing early leave a running
  attempt where it is, and so does an attempt whose deadline has already passed (judged on
  the database's clock), which is over whether or not it has been swept and is not revived.
  Each attempt moved is audited as `attempt.extended` with the old and new deadline and the
  cause. An override moves only the attempts of the students it
  names.
- §13 An attempt starts under the window as it stands when it commits. A start that is
  racing an extension stores the extended deadline, or is itself lengthened by the
  extension, whichever commits first; it is never left on the close it read before.
- §15 `closesAt` of an override must be ahead of the database's clock, not the API
  server's.
- §15 `MonitorRow.extendedTo` carries the time a student's own override closes when that
  is later than the assignment's effective close, and is absent otherwise. An override
  that closes sooner than the assignment says nothing. Closing an assignment early does
  not take an override away: a student whose override reaches past the close stays open.
- §13.9 The maintenance window's extension also moves the close of each student override of
  a published assignment that falls inside the window, by the window's length, and writes
  an `assignment.override_extended` audit entry per override. The operator's role needs
  `SELECT, UPDATE (closes_at)` on `assignment_student_overrides` (release checklist).
  `extendAssignment` and the two override writes take the same advisory lock (73819, 40)
  shared, as a start does.

**Changes since v0.64**

R4, extensions and per-student overrides, first half (T-R4.12a):

- §13 New table `assignment_student_overrides`, one row per assignment and student, that
  gives one student a later close, a longer time limit (1 to 600 minutes) and up to 10
  more attempts, with a required reason (1 to 500 characters). It cascades with the
  assignment and with the student, and a row must change something. Migration 00101.
  A missing row is the assignment as everyone has it.
- §15 `POST /teacher/assignments/:id/extend` moves the close later for everyone
  (`teaching.assignments.write`) and answers `409 ASSIGNMENT_CLOSED` for a closed
  assignment, which is reopened instead. `PUT`, `GET` and `DELETE` on
  `/teacher/assignments/:id/student-overrides[/:studentId]`
  (`teaching.attempts.intervene`) set, list and remove one student's override. A request
  names only students of the assignment the caller reaches; a student who is not one
  answers `422 VALIDATION_FAILED` for the whole request. A field the request names
  replaces the stored one and a field it leaves out keeps it. Every write is audited with
  the old and new values.
- §9 The student's card, intro, start and result read the student's own window: the later
  of the assignment's close and their override's (which lifts an early close), their time
  limit, and the attempts they were given. An assignment that closed for everyone else
  reads open to a student whose override is still ahead. A result released `after_close`
  is withheld until that student's own close; the class average still waits for the
  assignment's. Nothing in a student's payload says an override exists or why.
- An attempt already in progress kept its deadline in this version; v0.66 (T-R4.12b) recomputes it.

**Changes since v0.63**

R4, assignment review options, the note to students and the live lock (T-R4.11):

- §13 `assignments` gains `review_release` (`on_submit` or `after_close`, default
  `on_submit`), `review_show_class_average` (default false) and a nullable
  `student_note` (1 to 500 characters once trimmed). Migrations 00099 and 00100; every
  default is a constant, so nothing is rewritten.
- §14 `Assignment.review` carries `release` and `showClassAverage`, and `Assignment`
  carries `studentNote`. On a write the three are partial: omitted, they keep what is
  stored, and `studentNote: null` clears the note.
- §15 While an assignment is `open`, a PATCH that changes `testVersionId`,
  `durationMinutes` or `maxAttempts` answers `409 ASSIGNMENT_LOCKED`; nothing else locks.
- §9 A result released `after_close` is withheld until the assignment's effective close
  (score, marks, answer key, explanations and the grader's comments) and carries
  `releasesAt`. With the teacher's switch on, a released result of a closed assignment
  carries `classAverage` once at least 3 students qualify. The Test intro draws the
  teacher's note and says in its score sentence when the score comes.

**Changes since v0.62**

R4, the content editor's Rich text and Markdown modes (T-R4.64, DG-110):

- §7.1: rich prose is how a prompt and an explanation are written, no longer
  an opt-in behind `VITE_RICH_QUESTION_EDITOR`, which is removed. A field opens
  in the form it is stored in, and a field with no text opens as rich text;
  the builder's and the group composer's starter prompt was stored as Markdown
  until T-R4.31b and T-R4.34 created it as rich text (see v0.74).
  "Switch to Markdown" and "Apply conversion" are the only ways the stored form
  changes, each behind its confirmation; Markdown is refused while a
  fill-in-the-blank prompt holds a gap. Markdown gains GitHub's tables and
  `~~strikethrough~~`, for the teacher and for the student, and no other GFM
  syntax. Default, not yet confirmed by Thuong.

**Changes since v0.61**

R4, alt text for a question's image (T-R4.62a):

- A bank question whose media is an image carries optional alt text of 1 to 1000
  characters, returned to teachers as `mediaAlt` and frozen into the published
  version with the rest of the question, so a later bank edit never reaches a
  version. Students receive it beside `media` in the paper and in the result; it
  is a description for a screen reader, not part of the answer key.
- A value for a question whose media is not an image is refused with a field
  error on `mediaAlt` (400 `VALIDATION_FAILED`). A write replaces the stored
  value, so an update that leaves it out clears it. Restore as draft and a
  duplicated group carry it, and the version diff counts a changed alt text as a
  change to the question's media. Migrations 00097/00098 add nullable columns
  without backfill (`docs/plan/20-data-model.md` D-33, §37).

**Changes since v0.60**

R4, content editor frame (T-R4.63, DG-115):

- §7.1: a formatted paste leaves the images inside copied content out, unloaded,
  and counts them in its preview, instead of refusing the whole paste. Every
  other refusal stays; a paste holding only images, and a pasted or dropped
  file, are refused with a notice. Default, not yet confirmed by Thuong.

**Changes since v0.59**

R4, question metadata and authoring compatibility (T-R4.15):

- Teacher question reads and authoring carry nullable level and skill; draft test
  lists expose distinct sorted skills. Published metadata stays frozen through
  later bank edits and draft restoration.
- New authoring accepts at most eight choice options. Stored reads, bodyless
  copies, publication and restoration preserve legacy longer arrays. Migrations
  00087/00088 add nullable metadata without backfill. Local PG18 up/down/up,
  isolated app/migrate behavior and unchanged critical canaries pass. A separate read-only Neon data copy reports zero oversized
  questions among 64 bank and 23 frozen rows; no cloud DDL was executed.

**Changes since v0.58**

R4, import processing presentation (T-R4.37):

- The teacher processing page follows the deck for file and pasted-text sources,
  preserving real run stages, revision guards, cancellation, retention and review
  states. Text failure titles and recovery copy do not guess which source failed.
  Paste intake editing remains T-R4.57; run-event history remains unavailable.

**Changes since v0.57**

R4, account preference adoption (T-R4.7):

- Server-authoritative account preferences, reactive date displays and same-page
  actor/cookie/cleanup ordering are complete. Teacher wall-clock inputs remain
  fixed-HCM until T-R4.43; full profile controls and photo operations remain
  T-R4.43 and T-R4.8. This task completion is not a production release.

**Changes since v0.56**

R4, private caller profile and preferences (T-R4.7, staged):

- §7, §15 Public users gain optional display name/avatar; only the caller gains
  phone, locale, zone and preferences. Self profile PATCH preserves omitted keys
  and supports explicit clearing of display name/phone. Preferences merge top-level
  keys with bounded, closed payloads and atomic audits.
- §8 The Dashboard zone port now reads the caller's stored valid zone, defaulting
  only a stored NULL to Vietnam time. §13 records migrations00085/00086.
- Frontend account preference adoption, actor/cookie ordering and reactive date
  display remain the staged companion's acceptance gates. Existing datetime
  input parsing stays fixed to Vietnam time; full profile controls/photo follow
  T-R4.43/T-R4.8. No completed frontend or release is claimed by this record.

R4, private pasted-text sources (T-R4.55):

- §16 Adds plaintext exam intake, source character counts and text evidence using
  the existing private upload lifecycle. Recognition conventions and paste editing
  remain separate T-R4.56 and T-R4.57.

**Changes since v0.55**

R4, import history read model (T-R4.21):

- §16 History adds search-scoped status facets and nullable counts from the
  current review body. Repeated statuses are OR-ed; Processing includes all
  four active statuses. Legacy uncomputed counts remain unknown, and a pending
  machine candidate never replaces the counts of the teacher's current edits.

**Changes since v0.54**

R4, import history and source upload presentation (T-R4.36):

- §16 History uses one responsive card/grid with URL-backed filters and real
  pagination. File intake keeps selected files and the title across Source mode
  changes. Facets, text metadata and paste intake follow in T-R4.21, T-R4.55 and
  T-R4.57; their data and operations are not synthesized by the presentation.

**Changes since v0.53**

R4, read-only grading queue (T-R4.14):

- §8, §15 Teachers can read pending manual answers grouped by student or frozen
  question, with complete counts and a deterministic first200-item prefix.
  Historical paper reach and the existing explicit Finish operation are preserved.

**Changes since v0.52**

R4, teacher home and shell counts (T-R4.19):

- §5 Both home operations use the caller's own teaching scope, admins included;
  repository-wide reads and the attempt list retain their existing scope.
- §8, §15 The dashboard adds live taking counts, range-bound calendar submissions,
  today's assignment events and recent activity; the nav gets its own summary.
  Grading is permission-aware, calendar queries use the actor-zone port and
  application clock, and nullable graded scores retain their existing meaning.

**Changes since v0.51**

Round 5, the merged fixes and their limits:

- §9, §15 A tab names its held session on each attempt read; a superseded
  answer stays read-only and preserves another session's local draft (#337).
- §9 A deadline lock rearms automatic submission without replacing a closed
  or superseded lock (#353, #337).
- §10.1, §10.6 A question's blocked seek records `audio_seek` (#339).
- §10.4, §10.6 Sequence numbers use a server-calibrated offset and local
  increments; cross-tab order and uniqueness are not guaranteed. The timeline
  follows its session and sequence comparator (#338).
- §10.6 A shared recording reports its end and block; duration is elapsed
  device wall time since the latest start, with pairing limits (#340).
- §11.3 A blocked play keeps its player, and an aborted start shows no
  expired-link card (#339).
- §15 The body-limit reader answers `408 REQUEST_INCOMPLETE`; the earlier
  body-key reader can still hide an incomplete request (#342).
- §15 Every JSON-body operation declares the validator's 400 response (#346).
- §5.2 Own sign-out removes `next` in that tab's guard; the second-tab expired
  card remains open work (#355, #375).

**Changes since v0.50**

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §11.1 Audio may be 50 MB; an image stays at 10 MB, and the length stays at
  five minutes (DG-63). A teacher's media library has a quota, 5 GB by
  default. A file can be renamed in the library, audio can carry a default
  play limit, and an image's size in pixels is recorded at upload. §15 lists
  the search, the "not used" filter and `PATCH /teacher/media/:id`
  (T-R4.17a).

**Changes since v0.49**

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §6.1 At each start-up the API replaces every join code held only as its
  SHA-256, and neither revoked nor expired, with a sealed one that keeps its
  expiry and use cap, and tells the class's teacher once in the app. The old
  code then answers as any replaced code does (T-R4.22).

**Changes since v0.48**

A code nothing sent leaves the contract, and the sections are brought level
with six fixes to the web app:

- §15 `ALREADY_ENROLLED` is removed from `ErrorCode`: no server version ever
  sent it (#315).
- §15 A path outside the contract answers `404 NOT_FOUND` and a path under a
  method it does not serve answers `405 METHOD_NOT_ALLOWED` with `Allow`, both
  in the envelope. `HEAD` is answered as `GET` without a body on the operations
  that need no token (#308).
- §9 A draft a closed tab left behind is sent under the session that wrote it
  before "Tiếp tục làm bài" gives the student a new one. A refusal that can
  never become a save drops it; any other failure keeps it, and the attempt is
  not resumed (#330).
- §10.6 Leaving the engine while the attempt goes on sends the buffered events
  through the beacon and keeps the sequence (#331).
- §10.6 A question's audio player records `audio_ended` and `audio_blocked`
  beside `audio_play`. §10.4 A play with no recorded end shows no duration and
  is not called ongoing (#314).
- §10.1, §11.4 A question's play is counted by
  `POST /app/attempts/:id/audio-play`; the three audio events report it and
  change no count.
- §11.3 The total a player shows is the server's probed length when it is
  known (#321).
- §11.3 The player's file is
  `web/src/features/media/components/AudioPlayer.tsx`.
- §9 Once the server has said the time is up, the timer and the Submit
  dialog's time left read 00:00 (#329).
- §5.2 A sign-out closes the "Vui lòng đăng nhập lại" overlay if it was raised
  while the sign-out was on its way (#328).

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §5 In the teacher workspace the six content lists (tests, questions,
  question groups, media, imports, assignments) hold the caller's own rows for
  every caller, the Admin included: `scope.all` no longer widens them. It
  still reads and edits by id, and still widens the lists of classes, students
  and attempts and the dashboard. §16 says the same of the import history
  (DG-53, T-R4.54).

**Changes since v0.47**

The contract's descriptions, brought level with four fixes:

- §15 An error's `message` follows `Accept-Language` on every operation; the
  field sentences and publish violations the rules word do not yet (#284).
- §15 `publishTest` reports a question whose media file was deleted as a
  violation, not a 500 (#287).
- §15 A JSON body that repeats a member name is refused, unless the server
  filled a default into it (#288).
- §15 `createDraftFromTestVersion` answers `RESOURCE_REFERENCED` when a
  question of the draft's groups is still used elsewhere (#296).

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §6.5 The signed-in operations that hand out a credential are limited per
  signed-in user, after the permission check, and no longer per address: staff
  behind one address stop sharing a budget. §5.5 says the same of
  `openDocsSession` (T-R4.53).

**Changes since v0.46**

R3, "Student console" (v0.9.0, `docs/plan/73-r3.md`), the screens and the engine
as built (T-R3.5 to T-R3.11):

- §5.2 A refusal ends only the session its request was sent under, and a 401
  at app load keeps a sign-in that finished while the request was out (#317).
- §6.2 A signed-in student joins from the Join dialog on Classes and Home. A
  member is told so from the preview's `classId`; a member's join answers 200,
  and nothing sends `ALREADY_ENROLLED`. A lookup's answer is remembered for 30
  seconds.
- §6.5 The preview returns three fields, `classId` among them.
- §9 The student shell branches at 768px, with bottom tabs below it. The
  `/app` rows describe Home, the intro, the result, Classes (cards that are not
  links) and Settings (Profile, Sign-in, Appearance) as built. The engine has
  panes, a number strip or a question sheet, and the Submit dialog in place of
  the review page. Keys are A–E, F and the arrows, which stop at either end.
  §7.3's sentence on learner material says the same: one pane at a time on a
  phone, with no collapse state.
- §10.1 The clipboard listeners are on `document`.
- §10.2 The focus dialog's body is the deck's, without the seconds away or
  "the timer keeps running". The intro's start or resume click enters
  fullscreen; Home's resume card does not, and the engine's bar offers it. The
  engine leaves fullscreen once the attempt is submitted and records no
  fullscreen change after that. A blocked copy or paste raises a toast. The
  timer turns red under five minutes.
- §10.6 The monitor owns every listener that records or stops a signal. Other
  hooks listen to the same events and do neither: `useClipboardNotice` for the
  toast, `useSaveStatus` for the save line, `useVersionWatch` for a newer build.
  A question's audio player records `audio_play` through `recordAudioEvent`.
- §12 The student rules: one breakpoint, the page widths, the 44px floor and
  the controls that keep the deck's size, the dialog frame, the timer's digit
  cells, larger text in tests. The integrity UI follows the deck on the
  student side. The teacher console is the only one that still forces light.
- §7, §13.3, §15 v0.9.0 ships without R2's contract steps. The legacy `role`,
  the `NOT VALID` owner constraints with their fill triggers and the `/admin`
  alias stay until v0.9.1 (T-R3.1 to T-R3.3), no earlier than 2026-10-10.

**Changes since v0.45**

R3, "Student console" (v0.9.0, `docs/plan/73-r3.md`), the contract (T-R3.4 and
three fixes):

- §15 A student's assignment card carries `liveAnsweredCount`, and the result
  carries the paper's `sections` with a `sectionId` on every question
  (T-R3.4).
- §15 `startOrResumeAttempt` accepts `resume`; a Continue never starts an
  attempt (#237).
- §15 The monitor's `answeredCount` counts by the same rule as
  `liveAnsweredCount` (#234).
- §15 A student's assignment card carries `classIds` (#240).
- §15 A logout the maintenance gate or the limiter refuses, or whose revoke
  fails, still clears the session cookies (#188, #256).
- §5.2 A sign-in stores its refresh token under the user's lock and is refused
  if the account changed meanwhile; the rotations, reuse detections and
  logouts of one user run one at a time (#248, #279).
- §5.4 An individual target who is already on an assignment stays on it when
  disabled (#210).
- §15 `deleteQuestionGroup` answers `RESOURCE_REFERENCED` (#213).

**Changes since v0.44**

R2, "Access" (v0.8.0, `docs/plan/72-r2.md`):

- §1.1 Roles are data, one per user. The personas are the Admin (the owner),
  Teachers and Students; the Assistant role exists with no class staff yet.
- §1.3 One organization with several teachers. Every class, test, question,
  question group and media asset has an owner, and every Word import its
  creator.
- §5 Every operation that requires the bearer token declares `x-permission`,
  which `RequirePermission` enforces on every request from a 10-second
  principal cache; the nine open operations declare none. Another teacher's
  id answers as a missing one does.
- §5.2 The session epoch: disabling a student or resetting their password ends
  their live sessions at once, and a disabled user is refused on the next
  request.
- §5.3 Sign-in returns `CurrentUser`.
- §5.4 The web guards read the user's workspaces, not the role. The guards
  that are not permissions: strict student targets, the subset rule,
  disabling, shared students (`403 STUDENT_SHARED`) and the last Admin.
- §5.5 The API reference needs `system.api_reference`, and its cookie carries
  the session epoch.
- §6.1 New join codes are stored encrypted under `JOIN_CODE_KEY` and read back
  for their teacher. Codes issued before v0.8.0 stay hashed until R4. The key
  and its rotation are described.
- §6.4 The controls belong to the class's teacher, and to an Admin through
  `scope.all`.
- §6.5 Limits are sized for a classroom: 120/min and 600/h per address, 200/h
  per code, and the login, refresh and logout budgets. Keyed bodies take at
  most 8 KiB (`413`). Lookup is by a keyed hash.
- §7 Adds `CurrentUser`, `PermissionKey` and `Workspace`; `Role` is legacy.
- §13.2 `roles` is a table, not an enum. §13.3 adds `permissions`, `roles`,
  `role_permissions`, `users.role_id`, `session_epoch` and `created_by`, the
  owner columns and the join-code columns. §13.5 describes sealed join codes
  and the read-only access tables. §13.7 allows reference data in a migration.
- §15 Teaching operations move to `/teacher/*`, and the v0.7.0 `/admin/*`
  paths answer through an alias until R3 (v0.9.1, since v0.47). `/admin/*` keeps `deleteUser` and
  the docs session. Adds `PATCH /auth/me`, `getJoinCode`, the guards' 403s
  and `RESOURCE_REFERENCED`'s `details.referencedBy`.
- Word import: an import, its sources, review and commit belong to its
  creator, and `scope.all` reaches every import.

**Changes since v0.43**

R1, "Foundations and front door" (v0.7.0, `docs/plan/71-r1.md`):

- §4 The organisation's name and front-desk details come from build-time config.
- §5.2 A refused session keeps a signed-in user's page under a "sign in again"
  overlay. A refresh that fails without refusing is not a lost session.
- §5.4 New passwords need 8 characters and a number or symbol, and must differ
  from the current one (`PASSWORD_UNCHANGED`). `/forgot-password` exists.
- §6.1 The alphabet prose now matches the alphabet, which contains `L`.
- §6.2 Join previews the class inline on `/join/:code`, and ends on a Joined
  state. `/join/:code/confirm` redirects.
- §9 Covers `/forgot-password`, the system pages, the boot splash and the
  overlays (maintenance, sign in again, a newer build).
- §10.2 Integrity pauses under the maintenance and sign-in overlays. The
  engine adopts the server's deadline from every autosave.
- §12 Records the deck's foundations as built: tokens, Be Vietnam Pro, dark
  mode, `data-scale="deck"`, the accessibility tokens and the motion list.
- §13.9 Adds maintenance windows.
- §15 Adds `GET /public/status` and the 503 during a window, plus
  `MAINTENANCE_SCHEDULED`, `DEADLINE_NOT_REACHED`, `PASSWORD_UNCHANGED` and
  `saveAnswers.deadlineAt`.

**Changes since v0.42**

- §16 Word and PDF import run in production (O-24, decided 2026-09-25). The
  image ships the import worker, and `fly.toml` runs it as its own process group
  on a 1 GB Machine, which the API wakes over Fly's private network.

**Changes since v0.41**

- §16 Word import also takes PDF (D-10, decided 2026-09-25):
  - Only a PDF with a text layer can be read. A scan fails with `PDF_NO_TEXT`;
    there is no OCR. (Superseded in v0.99 for an import that asks for "Read with
    AI": the assistant reads the scan's pages, §16. There is still no local OCR.)
  - Underline, bold and colour are not read from a PDF, and every PDF draft says
    so. Keys come from a key file or explicit `1. A` lines.
  - PDFium reads the file inside a WebAssembly sandbox in the worker. No
    conversion runs.
  - `ImportSource.format` and `ImportLimits.formats` gain `pdf`.

**Changes since v0.40**

- §16 Word import has a retention policy (D-08, approved 2026-09-25):
  - Original files and the review draft are kept 30 days after the test is
    created and 7 days after cancellation.
  - An import untouched for 60 days is closed and its files removed.
  - History rows stay.
  - The API applies the policy at start-up and then daily. It exists wherever
    import storage does, with or without a worker.
  - `WordImport.filesRemovedAt` records the removal. The source download, the
    source view and the review then answer 410 `IMPORT_FILES_REMOVED`.
  - `GET /admin/imports/capabilities` states the three periods.

**Changes since v0.39**

- §16 Word import states whether it can run on this deployment:
  - `GET /admin/imports/capabilities` reports `intakeEnabled` and
    `processingEnabled`.
  - Processing requires `IMPORT_PROCESSING_ENABLED`, set beside a worker. Without
    it, `processWordImport` answers 503 `IMPORT_PROCESSING_UNAVAILABLE` and
    queues nothing.
  - The client hides or explains whatever cannot run.
  - Production keeps import off until O-24 is decided (#138).

**Changes since v0.38**

- §5.5 adds a docs session: the API reference (`/docs`, `/docs/openapi.json`)
  opens only with a fifteen-minute, docs-only cookie that an admin gets from
  `POST /admin/docs-session`. Its token is signed with its own key for its own
  audience and is never an access token. Local development may opt out with
  `DOCS_PUBLIC`, which production refuses. The page loads Scalar by SRI under a
  CSP that admits only that bundle and its own inline script (#135).

**Changes since v0.37**

- W-11b connects private storage, isolated conversion, chunked source extraction
  and deterministic recognition in a separate worker process. Completed stages
  survive retries; source/artifact checksums are verified before parsing. Run
  envelopes contain lineage IDs, with source and answer content in private artifacts.
  Public processing controls and full review/domain validation remain subsequent work.

**Changes since v0.34**

- W-13a adds an internal isolated legacy Word converter and private source-page
  rendition. One physical container slot, offline safe document loading, resource
  limits, cancellation and independent timeout protect processing capacity. Outputs
  retain conversion lineage and require review. Public legacy intake and durable
  processing integration remain disabled pending the complete pipeline.

**Changes since v0.33**

- W-12a adds ordered, source-bound extraction with Unicode fragment locations,
  nested/merged table evidence and explicit review reasons for private/ambiguous
  content. Bounded raster normalization retains images privately. These internal
  tools do not yet enable recognition, review or draft creation; original evidence
  is never a learner payload.

**Changes since v0.32**

- W-11 adds an internal durable processing queue with immutable source/pipeline
  identity, bounded attempts, leases, fenced writes and append-only run events.
  Cancellation and completion serialize on the import. A context-aware runner
  records safe operational metadata and preserves retryable work after shutdown.
  The production processor, supervisor and public processing controls are not
  wired yet; this checkpoint does not enable recognition or review.

**Changes since v0.31**

- W-10a adds disabled-by-default private native DOCX intake, teacher-only history,
  immutable source sets and retry identities. Storage writes have durable reservations,
  quota accounting and revision guards. Original downloads require teacher access and
  short-lived attachment URLs from a separate private bucket. Legacy conversion,
  processing, review, commit, retention and production acceptance remain later work.

**Changes since v0.30**

- W-07h integrates section → group → question authoring into the test builder.
  Outline and group writes share a serialized enclosing-test revision; acknowledged
  moves update only the editor's own revision baseline. Whole-group bank insertion,
  independent bank copies, complete learner preview and local recovery are available.
  The import pipeline and pilot acceptance remain release gates.

**Changes since v0.29**

- W-07g adds the independent group bank and a complete-group editor: shared
  materials, authorized image/audio insertion, stable gap links, learner preview,
  revision-checked autosave and explicit local recovery. Archived groups support
  bulk restoration/deletion; copies remain on the list with an indicator.
  Mixed builder editing and the import pipeline remain release gates.

**Changes since v0.28**

- W-07f saves complete mixed outlines under the test revision, moving whole owned
  groups between sections while retaining every existing group exactly once.
  Group removal remains explicit. Legacy saves clear obsolete unit rows after
  the last group is removed. Builder integration remains a release gate.

**Changes since v0.27**

- W-07e exposes teacher-only complete-group read/write/copy/archive operations,
  bounded bank summaries and ordered draft units. Writes check aggregate and
  enclosing-test revisions. Media-link outages return a retryable asset state
  without obscuring committed content. Builder integration remains a release gate.

**Changes since v0.26**

- Draft totals/tag filters include group members. Legacy question-only outline
  writes refuse grouped drafts atomically. Archived tests must be restored before
  draft restoration/default-version changes. Assignment introductions include
  shared recording policies from the assigned version and explain their scope.

**Changes since v0.25**

- W-09f supplies frozen shared context in learner results and both teacher
  grading modes. Transcript release follows each recording's policy independently
  of score/key/explanation flags. Review playback does not add attempt plays.

**Changes since v0.24**

- W-09e renders shared learner materials beside the question on wide screens and
  above it on phones with remembered collapse state. Stable recording players and
  idempotent gesture recovery preserve shared counts across member navigation,
  reload and takeover. Result/review context and authoring remain gated.

**Changes since v0.23**

- W-09d freezes the delivery algorithm on each published version. Historical
  standalone papers keep `section_v1`; group-aware snapshots use `group_v1`.
  Attempt reload, takeover and results select the frozen marker, never the
  current test default. Unsupported formats fail closed.

**Changes since v0.22**

- W-09c persists shared-recording counters and gesture receipts, deduplicates
  retries atomically with timeline events, and includes shared excess plays in
  teacher monitoring. Group authoring/player/result UI remain gated.

**Changes since v0.21**

- W-09b adds learner-safe shared context to start/resume/read attempt responses.
  Frozen media bindings require an attempt owned by the learner. Shared playback,
  result/review context and group authoring remain gated.

**Changes since v0.20**

- W-08c previews frozen group context and authorized assets without grading keys,
  follows the selected default version, and offers a 320px learner preview inside
  the supported admin shell. Group authoring and live learner delivery stay gated.

**Changes since v0.19**

- W-08b restores and duplicates complete independent context graphs, preserves
  mixed unit order and removes owned draft graphs when an unreferenced archived
  test is deleted. Grouped preview/draft UI and delivery remain gated.

**Changes since v0.18**

- W-08 defines independent frozen group membership, materials, gap targets and
  recording policies alongside existing version questions. Historical snapshots
  are not backfilled; group authoring stays unavailable until readers are ready.

**Changes since v0.17**

- W-07d adds revision-checked full-group editing, independent copy materialization,
  bank archive/restore/delete and section removal. These remain internal operations
  until complete draft, snapshot and learner readers are available.

**Changes since v0.16**

- W-07c adds atomic internal group persistence, owned-member barriers and protected
  media references. No group endpoint or authoring affordance is enabled yet.

**Changes since v0.15**

- W-07b adds relational group ownership, ordered units, stable cloze targets and
  protected material/recording bindings. Existing question ownership remains
  null; new group authoring is still unavailable until lifecycle and readers ship.

**Changes since v0.14**

- W-07a defines the bounded, independent group graph: ordered members, rich
  materials, explicit choice/blank gap targets and shared recording identities.
  Validation and detached copying precede persistence and delivery; group writes
  remain unavailable until snapshots, protected references and readers are ready.

**Changes since v0.13**

- Rich authoring supports bounded structured clipboard conversion with an explicit
  preview, atomic insertion and undo. Unsupported source content is refused in full;
  it is never silently reduced to plain text. Original clipboard HTML stays local.

**Changes since v0.12**

- Rich fill-blank prompts bind stable gap identities to answer rows. Moving gaps
  keeps answers attached; copies remap both ends; published versions preserve the
  frozen graph. Legacy Markdown and answer payloads remain compatible.

**Changes since v0.11**

- Manual/internal question writes and publication share interaction validation.
  Single-choice and true/false require one key; true/false requires two options.
  Question points are positive exact hundredths within numeric(8,2); publication
  sums exact hundredths and rejects empty exams or an overflowing total.

**Changes since v0.10**

- W-05b adds bounded semantic prompts and explanations with exact plain projections,
  immutable snapshots and policy-gated explanation delivery. Legacy Markdown remains
  on its existing reader; no old content is rewritten. Fill-blank prompts retain
  Markdown until stable gap binding is integrated.

**Changes since v0.9**

- Inline option formatting now has additive storage, snapshot/restore and learner
  readers (§7.1, §13.3). New authoring is a pilot opt-in; legacy options remain
  literal text and existing published rows are not backfilled.
- Option documents contain only marked text and line breaks; their plain text
  must match the stored `text`. Missing rich content in a legacy write cannot
  silently erase formatting. See `docs/plan/19-content-contract.md`.

**Changes since v0.8**

- The Word foundation defines an additive `ContentDocument` contract (§7.1).
  Existing question writes, stored Markdown and published versions are unchanged.
- Thuong approved independent copies of groups/materials and per-account local
  recovery for unsent edits, with seven-day expiry and logout clearing (§16).
- New grouped exams will shuffle whole groups within their section, retain member
  order, and share a recording's play allowance across its group (§7.2, §11.5).

**Changes since v0.7**

- The Word exam import milestone is accepted for implementation (§16). Its full
  scope includes semantic content, shared materials and groups, durable review,
  safe publication/delivery and operational release gates. It is not yet released.
- Both cloud and private AI processing remain candidates for measured evaluation;
  provider selection and external exam-data processing are not enabled by default.
  **Superseded by D19 (v0.99) for the AI assistant's features:** cloud processing is
  on, with the provider chosen and the data each feature sends decided (§2, §17).
  Private inference is still a candidate (`docs/plan/17-word-import.md` §8).

**Changes since v0.6**

- Dashboard work cards lead into a full-width assignment table and compact activity below.
- Student discovery has counted status filters and prominent resume cards.
- Teacher and student settings share Profile, Security and Preferences routes,
  persistent forms, section navigation and separated form rows.
- Subtle shadows, hover feedback and 150ms section transitions respect reduced motion.

**Changes since v0.5**

- Admin lists retain filters, support bulk removal and duplicate in place. The
  question bank exposes attached outlines in a nested table (§8).
- Version history supports safe draft restoration, default selection and unused
  version deletion without changing assigned snapshots or reusing version numbers.
- Assignment limits support custom durations, attempt counts and immediate
  integrity submission with retained answers and recorded violations (§10).

**Changes since v0.4**

- The approved independent UX review replaces the student mockup's sparse right
  panels with fluid discovery pages and centred reading/form pages (§9, §12).
- All live attempts remain discoverable; class/status filters, result empties,
  explicit selection/audio/save states and direct submitted-paper navigation are
  part of the student flow. Responsive changes preserve local form/filter state.
- Phone actions use 44px touch targets, including navigation and dialog controls;
  the final-question footer must fit at 320px in Vietnamese and English.

**Changes since v0.3**

- Pending student answers survive a failed save/reload within the same session;
  manual submission waits for the final save. Keyboard editing/navigation rules
  are explicit in §9.
- §12 pins the mockup type scale and phone input/touch exceptions.
- Approved O-23 defines thirteen-month integrity retention, retained audit logs
  and manual structured-identity anonymization in §13.3.

**Changes since v0.2**
- **OQ-2 answered: yes** — students self-join with a class code. New §6, new `/join` flow, join-code lifecycle in the schema. This adds a public, unauthenticated surface that did not exist before; read §6.5 on the security consequences.
- **OQ-3 answered: yes** — audio (listening) questions ship in v1, with upload and a custom player. New §11, new `media_assets` table, object storage added to the stack.
- **OQ-4 answered: yes** — `sample_answer` on `short_answer` questions, visible to the admin during grading only.
- All open questions are now closed. §17 lists the decisions that were made *inside* these answers and are worth a second look.

---

## 1. Product context

Quizzivy is a web app supporting a private English-teaching practice. Students take tests assigned by their teacher; the teacher/admin builds tests, assigns them, monitors attempts, grades open answers, and reviews results.

**v1 ships one capability end-to-end: test-taking.** Later versions add vocabulary practice, homework, pronunciation drills, and class management. v1 must not paint us into a corner for those, but must not build them either.

### 1.1 Personas

| Persona | Built-in role | Description |
|---|---|---|
| Owner | Admin | Runs the practice and teaches in it. Holds every permission ("Take tests" only when turned on) and reaches every teacher's data (`scope.all`). Works in the teacher console, and from R5 in the Admin console too. **Desktop/tablet only.** |
| Teacher | Teacher | Builds tests, manages students and classes, grades, reads results, over the classes and content they own. **Desktop/tablet only.** Data-dense UI is fine. |
| Student | Student | Joins a class with a code, takes assigned tests, sees own results. Often on a phone. Needs a calm, focused UI. |

Roles are data (`app.roles`), one per user (`users.role_id`). The four built-in roles are Admin, Teacher, Assistant and Student; their grants from the permission catalogue are in `docs/plan/70-redesign-overview.md` §4.1, and §5 says how they are enforced. The Assistant role exists with the deck's grants; until the deck draws class staff, no class has an assistant and no role picker offers the role (D14, 70 §1). No screen creates a role or a staff account before R5.

### 1.2 Goals (v1)

1. A student can join a class and complete an assigned test on a phone without losing work on refresh, tab close, or brief network loss.
2. The admin can author a test with mixed question types — including listening — in under 15 minutes without reading docs.
3. Every attempt is reviewable: answers, timing, score breakdown, and an integrity timeline.
4. Codebase is structured so the next feature (e.g., vocabulary sets) is a new folder under `features/`, not a rewrite.

### 1.3 Non-goals (v1)

- **Payments and subscriptions.** The AI assistant's credits (§13.10) are a budget that stops the assistant, not a payment: the product bills nobody and takes no card. What the centre spends with the provider is paid for outside Quizzivy.
- **An AI that decides.** The assistant suggests; a teacher accepts, edits or drops. Nothing it returns is saved as a grade, a question or a comment by itself, and no AI response reaches a student.
- **Open public sign-up.** Self-signup exists but is gated behind a class join code (§6). There is no "create an account" entry point without one.
- **Multi-tenant / multiple schools.** One organization with several teachers. Each class, test, question, question group and media asset has an owning teacher, and each Word import its creator (§5); the Admin reaches them all. No org/tenant scoping in the schema.
- **Hard proctoring** — webcam, screen recording, screen-lock, remote inspection. §10 covers browser-signal monitoring only; see §10.5 on its limits.
- **Audio transcoding, trimming, or waveform editing.** Upload, validate, serve. Nothing more (§11.2).
- **Speaking/recording questions.** Students listen in v1; they do not record. That is a separate feature with its own storage, consent, and grading model.
- **Rich analytics dashboards.** Per-test and per-student score tables only.
- **Native mobile apps.** Responsive web only.
- **Real-time collaborative authoring.** One person edits at a time.

---

## 2. Tech stack

Fixed unless Thuong approves a change. Do not swap libraries silently.

| Concern | Choice | Notes |
|---|---|---|
| Build | Vite + React 19 + TypeScript (strict) | `pnpm` |
| Routing | React Router v7 (SPA mode) | Route-level code splitting via `lazy` |
| Server state | TanStack Query v5 | All API reads/writes go through query/mutation hooks |
| Client state | Zustand (minimal) | Auth session, test-taking engine, UI prefs only |
| Forms | react-hook-form + zod | Zod schemas are the single source of validation and TS types |
| Styling | Tailwind CSS v4 + shadcn/ui (neutral/zinc base) | See §12 |
| Icons | lucide-react | One icon set only |
| i18n | i18next + react-i18next | `vi` default, `en` secondary. All strings via `t()` from day one. |
| Dates | date-fns + date-fns-tz | `Asia/Ho_Chi_Minh` default |
| HTTP | native `fetch` wrapped in `src/lib/api/client.ts` | No axios |
| Google auth | Google Identity Services | Authorization Code + PKCE, exchanged server-side (§5.3) |
| AI assistant | Anthropic's API, through the official Go SDK in `platform/anthropic` | Reached only through the ai module's `Completer` port, wired in `core`. Two tiers, fast and capable (the capable one reads images and PDF pages); each tier's model and its three prices are configuration (`AI_TIER_*`), and no model identifier is written in the code or the documents. Off unless `ANTHROPIC_API_KEY` is set. §8, §15, §16, §17 |
| Audio | native `<audio>` element + custom React controls | No wavesurfer.js, no howler. See §11.3 |
| Object storage | Cloudflare R2 (S3-compatible) | Audio and image assets. `aws-sdk-go-v2/s3` on the backend |
| Markdown | react-markdown + rehype-sanitize | Never `dangerouslySetInnerHTML` with server content |
| Drag & drop | `@dnd-kit/core` + `@dnd-kit/sortable` | Builder reordering only; lazy-loaded |
| Testing | Vitest + Testing Library; Playwright e2e; MSW mocks | See §14 |
| Lint/format | ESLint (typescript-eslint, react-hooks, jsx-a11y) + Prettier | CI fails on lint errors |

**Single SPA, four route trees.** `/teacher/*`, `/admin/*`, `/app/*`, and a small public tree (`/login`, `/join/*`, `/set-password`). Split at the route level so a student never downloads teacher or Admin code and an anonymous visitor downloads none of them. A pre-R4 teacher address under `/admin/*` redirects to the same path under `/teacher`; the console's own addresses (`/admin`, `/admin/users`, `/admin/roles`, `/admin/classes`, `/admin/ai`, `/admin/audit`, `/admin/api`, `/admin/settings/:section?` and `/admin/exports/:id`) stay for a user with the `admin` workspace. Anyone else gets the 403 page at them, except at `/admin` and `/admin/classes`, which are also pages of the teacher's tree and redirect there.

---

## 3. Repository layout

```
web/src/
├─ app/                      # shell: providers, router, error boundaries, guards/
├─ features/
│  ├─ auth/
│  ├─ join/                  # NEW: public class-code join flow
│  ├─ tests/                 # admin: authoring
│  ├─ question-bank/
│  ├─ media/                 # NEW: upload widget + asset picker
│  ├─ assignments/
│  ├─ attempts/              # admin: monitoring + grading
│  ├─ integrity/
│  ├─ students/
│  ├─ classes/
│  ├─ take-test/             # student: the engine
│  └─ results/
├─ components/
│  ├─ ui/                    # shadcn primitives
│  └─ shared/
├─ layouts/                  # TeacherLayout (shell/), StudentLayout, FocusLayout
├─ lib/                      # api/, i18n/, utils/, config.ts
├─ hooks/  stores/  styles/  main.tsx
```

**Feature folder convention** (`src/features/<name>/`): `api.ts`, `schemas.ts`, `components/`, `pages/`, `store.ts` (optional), `index.ts` (public exports only). Features import each other only via `index.ts`. `components/shared` never imports from `features/`.

---

## 4. Naming and branding

Display name **Quizzivy**, sentence case. Package `quizzivy-web`; Go module `quizzivy`. DB `quizzivy`, schema `app` (not `public`, §13.2). R2 bucket `quizzivy-media`. The literal string appears once, in `.env`; everything else reads `config.appName`.

The organisation's own details are build-time config: `VITE_ORG_NAME`, `VITE_ORG_FRONT_DESK_PHONE` and `VITE_ORG_FRONT_DESK_HOURS` (`web/src/lib/config.ts`). An unset value hides what would show it, such as the front-desk row on `/forgot-password`. From R5 the admin's organisation settings take precedence, and these values stay as the fallback.

---

## 5. Auth

Signing in (§5.1–§5.3) says who a user is; the user's role (§1.1) says what they may do. The permission catalogue, the built-in grants, the Admin wildcard, the hidden keys and the pseudo-keys (`self`, `workspace.teacher`, `workspace.admin`) are in `docs/plan/70-redesign-overview.md` §4.1. `PermissionKey` in `api/openapi.yaml` equals the rows of `app.permissions`, and the server refuses to start when the database lacks a key it was built with.

- **Enforcement.** Every operation that requires the bearer token declares `x-permission` in `api/openapi.yaml`: a key, a pseudo-key, or a list met by any one of its keys (70 §4.2). `httpx.RequirePermission` enforces it on every request, after `RequireAuth`; the path is not the gate. The server refuses to start when an operation declares nothing, declares a value outside the catalogue, or declares one that does not belong to its path tree (`/teacher/*`, `/admin/*`, `/app/*`, `/auth/*`, `/me/*`).
- **Open operations** do not require the bearer token, declare no permission and pass untouched. There are nine: login, Google sign-in, refresh, logout, `POST /join/preview`, `GET /public/status`, `GET /public/organization`, `GET /public/organization/logo` and the integrity beacon (`POST /app/attempts/:id/events`).
- **Refusals.** No valid token, an unknown or disabled user, or a token older than the user's session epoch (§5.2) → `401 UNAUTHORIZED` with `WWW-Authenticate`, which the single-flight refresh settles. An unmet permission → `403 FORBIDDEN`, with no detail.
- **The principal.** Each gated request resolves the caller's role, permissions, `disabled_at` and session epoch through an in-process cache that keeps a user for 10 seconds. A write on the same machine forgets the entry at once, so the change applies on that machine's next request and on any other within 10 seconds. Polling does not query the database on every request.
- **Scope is separate from permission.** Repositories filter by `access.Scope`: another teacher's id answers as a missing one does, and their rows never appear in a list or a count. A student is visible to every teacher who reaches them: a member of a class the teacher teaches, an account the teacher created, or an individual target of an assignment the teacher created. `scope.all`, which only the Admin holds, lifts the filter for a read or a write by id, and in the lists of classes, students and attempts. It does not lift it in the teacher workspace's six content lists (`listTests`, `listQuestions`, `listQuestionGroups`, `listMedia`, `listWordImports`, `listAssignments`, with their facets, tags and counts): those hold the caller's own rows for every caller, the Admin included, and for assignments that is those the caller created or that target a class the caller teaches (DG-53, T-R4.54). Both teacher home operations, `getDashboard` and `getTeacherSummary`, pass `.Own()`, so their figures cover only the caller's teaching, admins included; the repository retains its wide `scope.all` reading for the future Admin console. The Admin reaches another teacher's content by id and, from R5, in the Admin console.

### 5.1 Methods

Two, both landing on the same `users` row keyed by verified email:

1. **Email + password** — student accounts created by staff (`people.students.create`), and staff sign-in.
2. **Google Sign-In** — the primary path for students, and the **only** path for self-join (§6.3).

A user may have both. Linking rule: a Google sign-in whose ID token carries `email_verified: true` matching an existing user links to that user. An **unverified** email is rejected outright — no link, no create. This closes an account-takeover path.

### 5.2 Session model

- Access token: JWT, ~15 min, held **in memory** (Zustand). Never localStorage, never sessionStorage. It carries the user, its times and the session epoch (`sep`) and no role; a token minted before v0.10.0 also names one, which is ignored.
- Refresh token: opaque, rotating, `httpOnly; Secure; SameSite=Lax; Path=/auth` cookie. Stored server-side as a hash (§13.5).
- On 401 the client calls `POST /auth/refresh` once and retries. The refresh is single-flight.
- A **refused** refresh (401 or 403), or a second 401, ends the session.
  - A refusal ends only the session its request was sent under. When the store's access token is no longer the one the refused request carried (a sign-in or a refresh replaced it, or the user signed out), nothing is expired or cleared, and the 401 returns to its caller.
  - A user who was signed in keeps the page: the access token is dropped, the user stays, and the "Vui lòng đăng nhập lại" overlay covers the page (§9).
    - A sign-out closes that overlay if it was raised while the sign-out was on its way.
    - After the user's own sign-out, that tab's guard sends a visitor to
      `/login` without `next` until its next sign-in. The marker is in memory,
      so a reload does not preserve it. An expired session keeps its return
      path. A second tab left open after sign-out can still offer `next`
      through its expired-session card; that path remains unresolved.
  - "Đăng nhập" clears the session, keeps the answer drafts, and goes to `/login?next=<path>`.
  - While that overlay is up, a 401 returns to its caller without another refresh.
- A refresh that **fails without refusing** (a network failure, a 503 or another 5xx) is not a lost session. Waiting requests fail retryably and nobody is signed out.
- App load: `GET /auth/me`, under the boot splash (§9).
  - 401 → signed out, unless a sign-in finished while the request was out.
  - A network failure (no HTTP response at all, on `/auth/me` or on the refresh it triggers) → the splash's offline state, which retries.
  - Any other failure (a 5xx, a 503 other than `MAINTENANCE`, or a 4xx other than 401 and 403) → on `/` and the signed-in routes, the unexpected-error page with the server's `requestId`; on the public routes the splash fades out and the page renders as usual.
  - 503 `MAINTENANCE` → the maintenance overlay.
- Every request sends `Accept-Language` set to the app's locale, so server messages match the UI rather than the browser.
- Reuse detection: presenting an already-rotated token revokes the whole family and forces re-login.
- **Races.** A sign-in stores its refresh token under a lock on the user and compares the account with what it read: if the password, the session epoch or the disabled state changed meanwhile, a password sign-in answers `INVALID_CREDENTIALS` and a Google sign-in reads the account again. A rotation and a logout take the lock an update of the user takes, so one user's rotations, reuse detections and logouts run one at a time and none overlaps a reset, a disable or a password change: a session never survives the event that should have ended it.
- **Session epoch.** `users.session_epoch` ends live sessions at once. Sign-in, Google sign-in and refresh put the user's current epoch in the access token (claim `sep`; a token without it reads as 0), and a token older than the user's epoch is refused (§5). In R2 two writes move it, each in the same transaction that revokes every refresh family the student has: disabling a student (`updateStudent` with `disabled: true`) and resetting a student's password (`resetStudentPassword`, and each student of `resetStudentsPasswords`). Enabling the account again does not bring those sessions back. R5's sign-out-everywhere, role changes and set-password links move it too: using a link does, and so does issuing a reset to an account whose owner did not ask for it (70 §4.2).
- **Signed-in devices.** A session is a refresh-token family that is neither revoked nor expired. `GET /auth/sessions` returns the caller's own, at most 100, the one the request's refresh cookie names first with `current: true`: a handle (`familyId`), a `device` label ("Mac · Chrome", built at read time from closed lists of systems and browsers and never from the user agent's own text, or null), a `deviceKind` for the icon, a `location` ("City, CC", or null) and `lastUsedAt`. The location and the time are those of the latest sign-in or refresh, so the list shows where a device last refreshed, not where it first signed in: a rotation takes the label of its own request, and copies its predecessor's only when the request has none. The label is built from `CF-IPCity` and `CF-IPCountry` only when `CLIENT_IP_HEADER` is `CF-Connecting-IP` and the request carries that header; under any other configuration it is null. It reaches no log and no audit row. `DELETE /auth/sessions/{familyId}` ends one session, and `POST /auth/sessions/revoke-others` every one but the caller's. Each is one command: it takes the lock a rotation takes, revokes the family, moves the caller's session epoch, audits it and, after the commit, forgets the cached principal. A revoked device's access token stops working at once on that machine and within 10 seconds on any other; the caller's own next request answers 401 and recovers through the single-flight refresh, because the caller's family stays live and a refresh does not read the epoch. The caller's session is the one the refresh cookie names. A request whose cookie names no live session of the caller, such as one cleared by a sign-out in another tab or a bearer token alone, is answered `401 UNAUTHORIZED` by both writes, which write nothing; in the client that is a refresh that fails, and the sign-in overlay shows. An id that is another user's, revoked, expired or unknown answers `404`; the caller's own answers `409 SESSION_IS_CURRENT` (signing out is `POST /auth/logout`). A `revoke-others` that ends nothing writes nothing and moves no epoch.
- **A disabled user is refused on the next request.** Every gated request reads `disabled_at` through the principal cache (§5): at once on the machine that made the change, within 10 seconds on any other. Refresh refuses a disabled user and revokes the family, so the client's refresh ends the session.

### 5.3 Google flow

1. Frontend loads GIS and requests an **authorization code** (not implicit ID-token) with PKCE.
2. `POST /auth/google` `{ code, codeVerifier, redirectUri, joinCode? }`.
3. Backend exchanges with Google, verifies the ID token (`iss`, `aud`, `exp`, signature via JWKS), reads `sub`, `email`, `email_verified`, `name`, `picture`.
4. Resolution order:
   - identity exists → log in;
   - verified email matches a user → link identity, log in;
   - no match **and** a valid `joinCode` is present → create account + enroll (§6.3);
   - no match, no join code → `403 ACCOUNT_NOT_PROVISIONED`.
5. Returns `{ accessToken, user }`, where `user` is the `CurrentUser` (§7), and sets the refresh cookie.

`VITE_GOOGLE_CLIENT_ID` is public config. The client secret lives only in the backend.

### 5.4 Guards and edge cases

- `RequireSession` → `/login?next=<path>` when unauthenticated.
- The web guards read the signed-in user's `workspaces` and `permissions` (`CurrentUser`, §7), never the `role` record (it names the user's role on screen and decides nothing), through `features/auth/permissions.ts` (`can` and `useCan`, `hasWorkspace` and `useWorkspace`). They decide what the SPA shows; the server enforces the permissions on every request (§5).
  - `TeacherWorkspace` holds `/teacher/*`, the teacher console since R4, for the `teacher` workspace. Anyone else gets a **403 page**, not a redirect (a redirect hides the misconfiguration).
  - `AdminWorkspace` holds `/admin/*`, the Admin console since R5, for the `admin` workspace. A path under it that is not a console route is a teacher address from before R4 and goes to the redirect below. A console route for a user without the workspace answers the 403 page, except `/admin` and `/admin/classes`, which are also pages of the teacher's tree and redirect to `/teacher` and `/teacher/classes`.
  - `RequirePermission` sits inside it and above `AdminLayout`. It reads `handle.permission` of the leaf route (a key, a pseudo-key or a list met by any one of its keys, as `x-permission` is) and renders the 403 page when the user lacks it; a console route that declares none is refused too.
  - `StudentArea` holds `/app/*` for the `app` workspace. A user without it who has the `teacher` or `admin` workspace is redirected to their own console (`homePathFor`); a user with neither gets the 403 page.
  - `homePathFor` sends a built-in Admin (`scope.all`) who holds both consoles to `preferences.workspace`, `/admin` when it is unset; any other holder of the `teacher` workspace to `/teacher`; a user with only the `admin` workspace to `/admin`; and anyone else to `/app`. The switcher and the account menu write `preferences.workspace` when the user crosses to the other console. An Admin with "Take tests" turned on lands in the console they last chose and may also open `/app`.
  - A v0.9.x address under `/admin/*` that is not a console route redirects to the same path under `/teacher`, in any letter case, keeping its query and hash (`/admin/classes/:id` to `/teacher/classes/:id`, `/admin/settings/profile` to `/teacher/settings/profile`); `/admin/assignments/:id/attempts` and `/teacher/assignments/:id/attempts` go to the assignment's Students tab. The redirect stays, for the teacher's own bookmarks, until the release that removes the v0.7.0 alias.
  - `learnsOnly` (the student app is the user's only workspace) keeps v0.7.0's rules on `/join` and the Settings role label (§6.2).
- `mustChangePassword: true` → all routes redirect to `/change-password`. Google-only users never hit this.
- Logout: `POST /auth/logout` (revokes refresh token), clear store, `queryClient.clear()`, → `/login`. The server revokes the family under a budget of its own, ten seconds, that the end of the request does not cancel: a tab closed or a connection lost during sign-out still ends the session, whose cookie stays in a browser that never saw the answer. A refresh does the opposite and runs on the request's own context (`docs/plan/30-risks.md` R-06).
- The frontend applies accepted server locale/theme/display zone/larger
  text over browser mirrors, with omitted defaults `vi`/`light`/`Asia/Ho_Chi_Minh`/false
  and no default-materialization PATCH. Anonymous choices remain local until a sign-in
  whose account lacks that key: a valid device choice (one made on this browser, not a
  value copied from an account) is then applied and saved to the account once, and a
  failed save retries silently at the next sign-in. Authenticated
  controls preview pending changes, restore acknowledged presentation on failure and
  retain an explicit Retry. A browser-unsupported account zone retains its exact
  server value, reports compatibility and temporarily presents Vietnam time without
  saving that fallback. Actor departure invalidates stale asynchronous effects.
- The frontend orders refresh/login/Google-login/logout cookie sends and global
  draft cleanup before replacement admission. Refresh remains single-flight. A20s
  transport/admission UI deadline and10s cleanup deadline show pending status without
  abandoning raw ownership; late timed-out login never auto-admits. Permanent hangs
  may keep admission closed. Cross-tab ordering is not promised by this design.
- Password reset in v1: a holder of `people.students.reset_password` sets a temporary password (`word-word-word-NN`, three words and two digits, about 24.4 bits) from the student detail page, under the shared-student rule below, or for up to 40 students at once from the student list. From R5 an emailed set-password link (below) is another way to set a password; Admin and teacher actions issue it (T-R5.9, T-R5.14), and a Google-only account can ask for its own from Settings. There is still no reset form for an account to ask for one while signed out (§17.1).
- New passwords have three rules:
  - at least 8 characters;
  - at least one number or symbol (`[\p{N}\p{P}\p{S}]`);
  - different from the current password (the temporary one, while `mustChangePassword` is set).

  The contract enforces the first two (`400 VALIDATION_FAILED`) and the server the third (`400 PASSWORD_UNCHANGED`). Existing passwords are never re-validated. `/change-password` shows the rules and a strength meter.
- `/forgot-password` makes no request. It tells a Google user that no password is needed, and everyone else to ask the front desk (§4, when configured) or their teacher.
- **Set-password links** (R5, T-R5.3). A link carries a 256-bit token in the URL fragment (`/set-password#token=…`), which the page removes from the address before it sends anything, so the token reaches neither a server log nor a `Referer`. Only the token's SHA-256 is stored; the token itself waits, sealed, in the email outbox until the message is sent (`docs/plan/20-data-model.md` §49). Issuing a link revokes the account's earlier unused links, so one is live at a time, and it lasts 24 hours and works once. `POST /auth/set-password/check` names the account (`email`, `fullName`, `purpose`, `expiresAt`). `POST /auth/set-password` sets the password under the rules above (a weak one is `400 VALIDATION_FAILED` and spends the token's budget, not the token), clears `mustChangePassword`, stamps `passwordChangedAt`, revokes the account's other links and every refresh family, moves `session_epoch`, forgets the cached permissions and signs the account in, and audits `user.password_set` with the link's purpose. Two submits of one link set the password once; the other is `410`. An unknown, used, expired or revoked link and the link of a disabled account all answer the same `410 SET_PASSWORD_LINK_INVALID` with the same body, so a link says nothing about why it fails. Disabling an account revokes its unused links in the same transaction that revokes its refresh families, and enabling it again restores none: what was out in the world when access ended stays ended, and an Admin who wants the person back sends a new link through Reset. A token is refused before any password is hashed. While the centre has password sign-in off, using a link is `422 PASSWORD_SIGN_IN_DISABLED` before the token is looked at, an invite sends a sign-in invite and no link, and a reset is refused. A reset issued for someone else ends the account's sessions as every reset does; `POST /auth/set-password/request`, the signed-in account's own request from Settings, keeps the session it is made from, and using the link ends the others. A `purpose: invite` link is an account's first password; `reset` is any later one. A Google link is never removed by setting a password. The set-password page and Settings read the centre's public answer, asking the browser to revalidate it and keeping it for thirty seconds, so the page shows no form and Settings offers no link for a password the centre no longer accepts. The page waits for the answer three seconds and never changes its mind once it has decided; Settings draws the control until the answer says off. When the answer is late, fails or was stale, the form or the control shows and the server's `422` is the backstop.
- **Guards that are not permissions** (70 §4.3), enforced by the server. A guard on a student runs after the student is found in the caller's scope, so another teacher's student answers `404`, never `403`.
  - **Strict student targets.** Every student read and write (the Students list and record, update, reset, bulk reset and delete, class membership, individual assignment targets, the dashboard's counts and `maintenance anonymize-student`) accepts only a role in `app.student_like_roles`: the built-in Student, or a custom role granted nothing but `learning.take_tests`. An Admin with "Take tests" turned on is never a student target. For an assignment's individual targets the rule applies to a target being added: a student who is already a target stays on the assignment when they are disabled or their role stops being student-like, so the teacher can still publish, edit and close it (#210).
  - **The subset rule.** `updateStudent`, `resetStudentPassword`, `resetStudentsPasswords` (for each student in it) and `deleteUser` need the target's permissions, without `learning.take_tests`, to be a subset of the caller's (`access.CanActOn`); otherwise `403 FORBIDDEN`. 70 §4.3 lists the R5 operations it will also cover.
  - **Disabling.** `updateStudent` with `disabled`, either value, also needs `people.users.manage`; otherwise `403 FORBIDDEN`.
  - **Shared students.** A reset, single or bulk, or a new `email`, by a caller without `people.users.manage` needs a student no one else reaches: every class they are in, archived ones included, is the caller's; no other account created them; no other account's assignment targets them individually; and a student in no class was created by the caller. Otherwise `403 STUDENT_SHARED`, and nothing is written. Sending the address the student already has is not a change. A new address could take the account over through Google sign-in, which links by email (§5.1).
  - **The last Admin.** The `users_last_admin` trigger refuses any demotion, disable or delete that would leave no active Admin, whatever the path. No R2 operation can reach an Admin account; `409 LAST_ADMIN` arrives with R5.
  - The built-in Student role cannot lose `learning.take_tests`.

### 5.5 API reference session

- `/docs` and `/docs/openapi.json` are served by the API beside the contract and open only to a holder of `system.api_reference`, a hidden key only the Admin holds (70 §4.1). A browser navigating there sends no bearer token, so they check a separate cookie instead.
- `POST /admin/docs-session` (`openDocsSession`, `x-permission: system.api_reference`, rate-limited per signed-in user, §6.5) sets `quizzivy_docs`: `Path=/docs; HttpOnly; Secure; SameSite=Strict; Max-Age=900`. Its value is a JWT for the `docs` audience that carries the caller's session epoch, signed with a key derived from the access-token key: a docs token is never accepted as an access token, and an access token never opens the docs.
- `GET /admin/api-reference` (`getApiReferenceSummary`) and `GET /admin/api-reference/openapi.json` (`downloadOpenApi`), both `x-permission: system.api_reference`, rate-limited per signed-in user and reached with the bearer token, not the docs session, back the console's API reference page. The first reports the embedded contract's OpenAPI and document versions, its operation count and the count under each tag, read once per process; the second downloads `api/openapi.yaml` converted to JSON unchanged (`server/gen/contract/openapi.json`, written by `make gen`: the operation ids and descriptions as authored, not the PascalCased embedded spec the docs page renders) as `quizzivy-openapi.json` with `Cache-Control: no-store`. The body is declared as a byte stream so the generated server writes the JSON as it is.
- The gate resolves the cookie's user through the same principal cache as the API (§5). A missing, tampered or expired cookie, an unknown or disabled user, or a cookie older than the user's session epoch → `401`; a role without `system.api_reference` → `403`, both in the error envelope with no page or contract leaked. `POST /auth/logout` clears `quizzivy_docs` together with the refresh cookie, so signing out also ends an open docs session.
- The SPA's admin settings open the reference: the new tab is opened synchronously in the click, `opener` is cleared, and only then is the session requested and the tab pointed at `/docs`.
- `DOCS_PUBLIC=true` skips only the cookie check, for local development. The server refuses to start with it when `APP_ENV=production`; SRI, the page's CSP and the rate limit apply everywhere.

### 5.6 Organization settings and the sign-in policy

- One settings document for the installation (`app.organization_settings`, `docs/plan/20-data-model.md` §51): **profile** (`name` 1 to 120, `shortName`, `website`, `contactEmail`, `frontDeskPhone`, `frontDeskHours`, `address`; an optional text empty after trimming is `null`), **logos** (light and dark), **sign-in** (`google`, `password`, `forcePasswordChange`, `joinCodes`, `allowedGoogleDomains`) and **retention** (`keepAnswersMonths`, `keepFocusEventsMonths`, `anonymiseDisabledAfterMonths`). `GET /admin/settings` reads it and `PUT /admin/settings` replaces it, both `system.settings.write`, Admin only. A save names the `expectedRevision` it read and answers `409 STALE_WRITE` when someone saved since; it writes an `organization.updated` audit row in the same transaction, naming the changed fields and no values. A save that changes nothing writes nothing: the revision stays and no audit row is added.
- **Field rules.** The name and short name hold no line break, other control character or invisible (format) character. A website is a domain (`quizzivy.com`) or an `https` address, with no spaces and no credentials; `http`, other schemes and a bare word are refused. A contact email is a local part and a domain with a dot, as an address and nothing wrapped around it.
- **The front door.** `GET /public/organization` (open, `Cache-Control: public, max-age=300`, 120 a minute and 2,000 an hour per address) returns only what the sign-in page prints: name, short name, website, contact, hours, address, the logo URLs, which of Google and password sign-in are on, and whether joining by code is on. It carries no counts, plan, domain list, retention value or revision.
- **Logos.** `POST /admin/settings/logo` (multipart `{file, variant}`, 1 MiB, 20 an hour per signed-in user) checks and stores a logo content-addressed at `branding/{sha256}.png`; it applies only when a `PUT` references it. The server accepts a PNG and nothing else: it is square within 2%, 256 to 2048 px a side, decoded under the shared image gate and encoded afresh, which drops its metadata. An SVG answers `415 MEDIA_TYPE_UNSUPPORTED`, because proving an SVG cheap to draw means resolving its references as three browsers do; the settings page instead draws the Admin's SVG to a 1024 x 1024 canvas in their browser and uploads the PNG (D-019). `GET /public/organization/logo` serves the bytes as an attachment under `nosniff`, immutable when `v` is the current hash. Replaced and abandoned logo objects stay in the bucket: there is no sweep, growth is bounded by the upload limit, and identical bytes are one object. The database still admits the type `image/svg+xml`, a value the application never writes and reads as no logo.
- **The switches.** Password off answers `403 PASSWORD_SIGN_IN_DISABLED` to every password sign-in alike; Google off answers `403 GOOGLE_SIGN_IN_DISABLED` to sign-in and linking; a non-empty domain list refuses a Google email outside it with `403 GOOGLE_DOMAIN_NOT_ALLOWED`; unlinking the last way in answers `409 LAST_LOGIN_METHOD`. "Force password change" decides `mustChangePassword` for every temporary password issued afterwards, and leaves existing flags alone. Joining by code off makes `/join/preview`, `POST /app/classes/join` and Google-with-a-code answer exactly as an unknown code does.
- **Lockout.** A save that would leave the Admin saving it, or every active built-in Admin, with no way in is refused with `422 SIGN_IN_POLICY_LOCKOUT`, judged inside the save's transaction. `POST /admin/settings/sign-in-impact` previews the judgement and writes nothing.
- **Propagation.** Each server keeps the document for at most 30 seconds and replaces it at once when it saved, so another server serves the old policy for up to 30 seconds after a change.
- **The settings page.** System settings is `/admin/settings/:section` in the Admin console: Organization (also the bare address) and Sign-in, with Data & privacy and Plan & usage to follow (an address for a section that has not shipped is replaced by the bare one). The sections edit one form over the whole document and share one unsaved-changes bar; they stay mounted, so an edit survives a visit to another, and leaving the page with unsaved changes asks first. Save changes sends the whole document at the revision the form was last reset from, never a newer one that a background read has seen, so a save made elsewhere answers `409 STALE_WRITE`: the bar shows the server's message with Reload, which reads the document again and replaces the form. A rule the server refuses (`400 VALIDATION_FAILED`) is shown on its field and opens the section that holds the first such field. Once a save succeeds the front door is read again past the browser's five minutes (`GET /public/organization` with `cache: no-cache`, then every query under its key is invalidated), so the sign-in page and the teacher join-code callout follow the setting at once.
- **The Sign-in section.** The four switches and the Google domains, typed as chips: Enter, a comma or a space adds one, which is kept lower-case, without a leading `@`, once; Backspace in the empty input removes the last. While the edited policy differs from the saved one, and has a method on and only valid domains, the page asks `POST /admin/settings/sign-in-impact` a moment after the last edit and warns "{n} người sẽ không đăng nhập được nữa." (with the Admins among them). A policy with no method, or one the server counts as locking out the caller or every Admin, pauses Save changes, which stays focusable and says why in the bar and under the card; a `422 SIGN_IN_POLICY_LOCKOUT` from a save is shown the same way until the policy changes.
- **The logo dialog.** "Thay logo" takes a required logo for a light ground and an optional one for a dark ground, each a PNG of at most 1 MiB or an SVG, square within 2% and at least 256 x 256. A file the browser can tell is unusable is refused on its field before anything is sent; an SVG is drawn to a 1024 x 1024 canvas and sent as the PNG it exports (D-019); a refusal from the server shows its message on the field until another file is chosen. A stored logo shows in the Organization tile at once and applies when the settings are saved.

---

## 6. Class join codes (self-signup)

### 6.1 Model

A join code belongs to a class and is a **bearer secret**: whoever holds it can enrol. Treat it accordingly.

- Format: 8 characters from an unambiguous alphabet `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (no `0/O`, `1/I`). Displayed grouped `XXXX-XXXX`; accepted with or without the dash, case-insensitive.
- Generated from a CSPRNG. Never sequential, never derived from the class ID.
- Per-code controls: `expires_at` (default 30 days, counted to 23:59:59 of the last day in the issuer's calendar zone), `max_uses` (default null = unlimited), `uses_count`, `revoked_at`.
- One **active** code per class at a time. Rotating issues a new code and revokes the old one; previously enrolled students are unaffected.
- Stored encrypted, so it can be read back for its teacher and for admins (D5), and found by a keyed hash (§6.5). A code issued from v0.8.0 on is sealed under the server's `JOIN_CODE_KEY`, which the database never holds (§13.3). A code issued before v0.8.0 was held only as its SHA-256: it redeemed but could not be read back. v0.10.0 replaced every such code that was neither revoked nor expired, once, when the API started: `jobs.RotateLegacyJoinCodes`, which `Serve` runs beside the prunes and nothing else runs, under a two-minute budget and never on a timer. Each class is its own transaction, taking advisory lock `(73819, 41)`, then the class row, then re-reading the class's active code, so machines that start together rotate a class once and a second run finds nothing; a class that lost a deadlock is tried once more after the others, and a class the run could not reach waits for the next start-up. The sealed code keeps the old one's expiry, use cap (a stored NULL stays NULL), use count and creator, so a code that had reached its cap is used up again and the rotation admits no student the old code refused (F-67); it leaves the class's self-join setting as it is, and is audited as the System (`class.join_code_rotated`, reason `legacy_rotation`, hints only, never a code). Archived classes are rotated too. Each class's teacher is told once in the app (`join_codes.rotated`, with how many of their classes and the first five names); the old code then answers as any replaced code does, "revoked" where self-join is open and as an unknown code where it is closed or the class archived. A revoked or expired scheme-1 row is left as it is, and the scheme-1 lookup that finds it stays until R5 removes it (`docs/setup/operations.md`, "Legacy join codes").
- The key is standard base64 of exactly 32 random bytes, kept as a Fly secret with an offline copy. The API refuses to start without it and never logs it. HKDF-SHA256 derives three values from it, each under its own label: the AES-256-GCM key, the HMAC-SHA256 lookup key and a 16-bit key id; a key whose id derives to 0 is refused.
- Rotating the key: one deploy sets `JOIN_CODE_KEY_PREVIOUS` to the old key and `JOIN_CODE_KEY` to the new one, and codes under either key redeem and read back. `maintenance rekey-join-codes -apply` then re-seals every code under the old key with the new one, and the old key is unset (`docs/setup/operations.md`, "Join-code key"). A lost key leaves every code sealed under it unreadable and unredeemable: rotate every class's code.

### 6.2 Student flow

```
/join                → enter code; at 8 valid characters the class is previewed inline
/join/:code          → deep link (QR / message): code prefilled and previewed
                     → preview card: class name + teacher, "Tham gia {class}"
                     → signed out: sign in (Google → POST /auth/google {code, joinCode};
                       or password → POST /app/classes/join after sign-in)
                     → signed in: POST /app/classes/join at once
                     → Joined: "Bạn đã vào lớp {class}", "Đến lớp của tôi" → /app/classes
/join/:code/confirm  → redirects to /join/:code
/app, /app/classes   → "Tham gia lớp" opens the Join dialog for a signed-in student:
                       the same field and preview, then POST /app/classes/join
```

The preview exists so the student sees **which class they are joining** before authenticating. Never create an account and enrol in one blind tap.

- **Typing.** The field uppercases, drops spaces and dashes, and shows a dash after four characters. A character the alphabet never uses (`0`, `O`, `1`, `I`) marks the field invalid and sends nothing.
- **Lookup.** One lookup runs per complete code, after a 250 ms debounce. Every failure (invalid, expired, exhausted, revoked) shows the same message, and nothing about any class reaches the page. An answer is remembered for 30 seconds, a refusal as much as a class, so retyping a code sends nothing. An edit does not abandon a lookup already out, which the server has counted. A lookup that could not be made is sent again only when the code is retyped.
- **Signed out.** The join context (code, class name, teacher) is kept for 30 minutes in `sessionStorage` across sign-in. A must-change-password account goes through `/change-password` first, and the join then continues.
- **In the app.** A signed-in student joins without leaving the page. "Tham gia lớp" on Classes, and on Home for a student in no class with nothing assigned, opens the Join dialog: the same field and lookup, the class found (name and teacher) and "Tham gia", which calls `POST /app/classes/join`. On success the class and assignment lists refetch, a toast says "Bạn đã vào lớp {class}" and the dialog closes; it waits for the class list first, for at most three seconds. A 429 shows the server's message with the wait from `Retry-After`. The typed code and the state of a join are forgotten when the dialog closes. `/join` and `/join/:code` stay for links and QR codes.
- **Already a member.** The dialog compares the preview's `classId` with the student's own classes (`GET /app/classes`): a member reads "Bạn đã ở trong lớp này rồi." and cannot join. The server checks the code's state before membership, so a member who types a dead code reads the failure message. The enrolment is idempotent: a member's join answers 200 with the class and counts no use of the code, so on `/join` it reads as joined.
- **Outcomes.** On `/join` an enrolment failure shows its message on the Joined page. In the dialog a code that died after the preview shows the failure message, and any other failure "Không thể tham gia lớp. Vui lòng thử lại." An account that is not a student is told that joining is for student accounts.

### 6.3 Why Google-only for self-join

Self-signup with email + password requires a verified email, which requires transactional email infrastructure (provider, domain auth, deliverability, bounce handling) — a real dependency v1 does not have. Google hands us a verified email for free and is one tap on a phone, which is exactly the self-join case.

So: **self-join requires Google.** Password accounts remain staff-created. If Thuong later wants password self-signup, add an email provider and a `email_verifications` table; the join flow itself does not change. This is a v1 scoping decision, not a permanent constraint (§17.1).

### 6.4 Teacher controls

A class's code and members are managed by its teacher (`classes.teacher_id`), and by an Admin through `scope.all` (plan 70 §4.1). Another teacher's class answers 404, as a missing class does. The code operations and member changes need `teaching.classes.write`; the member list needs `people.students.read`.

On `/teacher/classes/:id` (the v0.7.0 path `/admin/classes/:id` redirects to it):
- Show the active code in full, with copy button, QR code, expiry, and uses count. `getJoinCode` reads the code back (§6.1), so the screen shows it whenever the teacher opens it, not only once when issued. The class cards show every code from one call, `listClasses` with `withCodes=true`, which opens a code exactly when `getJoinCode` would.
- **Rotate code** (confirm dialog: "Mã cũ sẽ ngừng hoạt động ngay").
- An **archived** class takes no new code: Rotate answers `409 CLASS_ARCHIVED` and leaves its current code as it is, because nobody can join an archived class (the code answers as an unknown one). Revoking stays allowed; restoring the class lets it rotate again.
- **Disable self-join** toggle — revokes the code without issuing a new one.
- Member list shows `joined_via` (`admin` / `join_code`) and `joined_at`, so the teacher can spot unexpected enrolments.
- Remove member (revokes access; attempts are retained, not deleted).
- **While the centre has turned joining by code off (§5.6, D-023)** the card does not hide: the "Students can join" switch and New code (Create code on a class without one) stay but are disabled, with "Trung tâm đang tắt tham gia bằng mã." / "Your centre has turned off joining with a code." as their reason; an existing code stays visible and is marked inactive; Stop stays. The Classes list does the same on its cards and menu, and its page lead says teachers add students instead of promoting the code. The callout above them is a row of its own, and names the way forward: a link to `/admin/settings/sign-in` for a holder of `system.settings.write`, "Hãy hỏi quản trị viên." / "Ask your admin." for anyone else. The server's writes stay allowed, because the per-class switch is the teacher's own state under the centre's rule and the redeem paths already answer as an unknown code. The rule reaches what hands the code on (D-028): Copy code, Copy link, Download QR and the QR on the card, and Copy link and Show QR on the list's cards and in their menu, are disabled with the same reason (the code stays visible, and Stop still works); New class creates the class, still open to joining, without a code, and its toast says the centre has joining off; its hint, the empty list and a class's empty Members text speak of adding students, not of a code. While the answer has not arrived, or failed, a screen behaves as if joining were on.

### 6.5 Security requirements — non-optional

A leaked code lets a stranger into the class. Mitigations, all required:

- **Rate limit** the sign-in and join operations, and return `429` with `Retry-After`. Without this, an 8-char code space is still worth probing at scale. The budgets let a class of 40 behind one school address make three tries at each step (T-R2.15):
  - `POST /join/preview`, `POST /app/classes/join` and `POST /auth/google`: 120/min and 600/h per address; 200/h per code, from any address. Google sign-in counts a code only when the body carries a `joinCode`.
  - `POST /auth/login`: 120/min and 600/h per address; 10/min per address and email; 20/h per email.
  - `POST /auth/refresh` and `POST /auth/logout`: 120/min and 1,200/h per address.
  - `POST /auth/set-password/check`: 120/min and 1,000/h per address; 20/h per token. `POST /auth/set-password`: 60/min and 600/h per address; 10/h per token. A token is counted as sent, so a wrong guess spends the budget of the string it guessed and a password that fails the rules spends the token's. A class of 40 behind one address opens its links together without meeting either limit.
  - `DELETE /auth/sessions/{familyId}`: 10/min and 60/h, and `POST /auth/sessions/revoke-others`: 5/min and 30/h, per signed-in user. Neither hands out a credential; each moves the user's session epoch and so makes every device of the user refresh (T-R4.9).
  - A code is counted after normalization, so respelling it buys no fresh allowance. Guessing stays at 600 codes an hour per address on each operation that checks one.
  - The signed-in operations that hand out a credential are limited per signed-in user, after the permission check and wherever the user connects from, so staff behind one address do not share a budget (T-R4.53): `resetStudentPassword` 5/min and 30/h, `resetStudentsPasswords` 2/min and 10/h, `createStudent` 30/min and 300/h, `createUser` 30/min and 300/h (T-R5.9a), `rotateJoinCode` 10/min and 60/h, `getJoinCode` 60/min and 600/h, `listClasses` 60/min and 600/h (it can open every code of a page), `openDocsSession` 5/min and 30/h, `requestSetPasswordLink` 3/min and 10/h. A caller with no session or without the permission is answered `401` or `403` and spends no budget.
- **Bounded bodies.** The six operations with a bucket keyed on a body field (login, Google sign-in, the preview, the in-app join and the two set-password operations) accept at most 8 KiB and answer `413` before any handler, so padding cannot hide the key from its bucket.
- **Constant-time comparison** on code lookup; look up by a keyed hash (HMAC-SHA256) of the normalized code, not by plaintext equality. A live code issued before v0.8.0 was found by its SHA-256 until v0.10.0's start-up rotation replaced it (§6.1); a revoked or expired one still is, until R5 removes the lookup.
- No audit row or log line carries a code, its ciphertext or its hash.
- `POST /join/preview` returns exactly three fields: the class's id, its name and the teacher's display name. Never student names, counts, a member list or assignment titles. The id is a uuidv7: it reveals only when the class was created and grants nothing by itself. The Join dialog uses it to tell a student who is already a member (§6.2). It is an unauthenticated endpoint.
- Log every enrolment (`class_id`, `user_id`, `ip`, `user_agent`, `at`) to the audit table.
- Expiry defaults to 30 days precisely so an abandoned code stops working on its own.

**Deliberately not built:** an approval queue. For a small practice, rotate-and-remove is sufficient and one less state machine. Revisit if enrolment volume grows (§17.2).

---

## 7. Domain model (frontend types)

Mirrors §13. IDs are UUID strings; timestamps ISO 8601 UTC.

```ts
interface User {
  id; email; fullName;
  displayName?: string;
  avatarUrl?: string;
  hasPassword: boolean;                 // false for Google-only accounts
  linkedProviders: ('google')[];
  mustChangePassword: boolean;
  createdAt;
}

type PermissionKey = 'content.tests.write' | /* … */ 'system.leads';  // the 22 keys of 70 §4.1, in catalogue order
type Workspace = 'teacher' | 'admin' | 'app';

interface CurrentUser extends User {    // the signed-in user only; a payload about anyone else carries User
  role?: CurrentUserRole;               // the role the user holds, for display; never read to decide what the user may do
  permissions: PermissionKey[];         // the role's effective keys, in catalogue order
  workspaces: Workspace[];              // derived from permissions; the web guards read these (§5.4)
  phone?: string;
  locale?: 'vi' | 'en';
  timeZone?: string;
  preferences?: UserPreferences;
}

interface CurrentUserRole {             // on the caller's own document only; the server always sends it
  id: UUID;
  builtinKey: 'admin' | 'teacher' | 'assistant' | 'student' | null;   // null for a custom role
  name: string;                         // the stored name, 1 to 60 characters
}

interface UserPreferences {
  theme?: 'light' | 'dark' | 'system';
  workspace?: 'teacher' | 'admin';      // the console the user last switched to; a built-in Admin opens it after sign-in
  compactTables?: boolean;
  largerTestText?: boolean;
  assignmentDefaults?: {
    durationMinutes?: number;
    shuffleQuestions?: boolean;
    showScore?: boolean;
    blockCopyPaste?: boolean;
    requireFullscreen?: boolean;
  };
}

interface Class {
  id; name; description?;
  scheduleLabel?: string | null;        // 1 to 120 characters
  room?: string | null;                 // 1 to 60
  studentCount: number;
  selfJoinEnabled: boolean;
  joinCode?: { hint: string; expiresAt: string; maxUses: number | null; usesCount: number } | null; // teacher responses only; the code itself comes from getJoinCode, or from listClasses with withCodes (§6.4)
  averageScore?: AttemptScore | null;   // teacher responses only (T-R4.18)
  createdAt;
}

interface MediaAsset {
  id; kind: 'image' | 'audio';
  url: string;                          // short-lived signed URL
  mimeType: string; bytes: number;
  durationMs?: number;                  // audio only
  originalFilename: string;
}

type QuestionType =
  | 'single_choice' | 'multiple_choice' | 'true_false'
  | 'fill_blank' | 'short_answer';

type QuestionLevel = 'pre_a1' | 'a1' | 'a2' | 'b1' | 'b2' | 'c1' | 'c2';
type QuestionSkill = 'grammar' | 'vocabulary' | 'reading' | 'listening' | 'writing' | 'speaking';

interface AudioPolicy {
  maxPlays: number | null;              // null = unlimited
  allowSeek: boolean;                   // default false for listening
  showTranscriptAfterSubmit: boolean;
  allowPause?: boolean;                 // default true; the server always sends it (§11.1)
}

// What a student receives: the same four fields, all present.
interface StudentAudioPolicy extends Required<AudioPolicy> {}

interface Question {
  id; type: QuestionType;
  level: QuestionLevel | null;
  skill: QuestionSkill | null;
  prompt: string;                       // Markdown, rendered sanitized
  media?: MediaAsset;
  mediaAlt?: string;                    // image only; frozen at publish; read to students by a screen reader
  audio?: AudioPolicy;                  // present iff media.kind === 'audio'
  transcript?: string;                  // admin-authored; student sees it only per policy
  options?: { id; text; isCorrect: boolean }[];
  blanks?: { id; ordinal: number; acceptedAnswers: string[]; caseSensitive: boolean }[];
  points: number;
  explanation?: string;
  sampleAnswer?: string;                // short_answer; ADMIN ONLY, never in a student payload
  tags: string[];
}

interface Section { id; title; instructions?; questionIds: string[] }

interface Test {
  id; title; description?;
  sections: Section[];
  totalPoints: number;                  // server-computed
  status: 'draft' | 'published' | 'archived';
  currentVersion: number;
  nextVersion: number;                  // server-computed: the number the next publish takes (last published + 1), whatever was deleted
  createdAt; updatedAt;
}

interface IntegrityPolicy {
  requireFullscreen: boolean;
  blockCopyPaste: boolean;
  maxFocusLoss: number;                 // -1 = none allowed; 0 = unlimited
  onLimitExceeded: 'warn' | 'flag' | 'auto_submit';
}

interface Assignment {
  id; testId; testVersionId; testVersion: number;
  targets: { classIds: string[]; studentIds: string[] };
  window: { opensAt: string; closesAt: string };
  durationMinutes: number;              // server-enforced
  maxAttempts: number;
  shuffleQuestions: boolean; shuffleOptions: boolean;
  review: {
    showScore: boolean; showCorrectAnswers: boolean; showExplanations: boolean;
    release: 'on_submit' | 'after_close';   // when a result is released; on a write, omitted keeps the stored value
    showClassAverage: boolean;              // whether the result may carry classAverage; omitted on a write keeps the stored value
  };
  integrity: IntegrityPolicy;
  studentNote: string | null;           // the teacher's note on the Test intro; plain text, trimmed, at most 500 characters (counted before trimming on a write); null clears
  status: 'scheduled' | 'open' | 'closed';
}

type AttemptStatus = 'in_progress' | 'submitted' | 'timed_out' | 'graded' | 'voided';

interface Attempt {
  id; assignmentId; studentId; testVersionId; attemptNo: number;
  status: AttemptStatus;
  startedAt; deadlineAt; submittedAt?; gradedAt?;
  answers: Record<string, Answer>;
  audioPlays: Record<string /*questionId*/, number>;   // server-authoritative (§11.4)
  score?: { earned: number; total: number; pendingManual: number };
  integrity?: { focusLossCount: number; flagged: boolean };
}

type Answer =
  | { type: 'choice'; optionIds: string[] }
  | { type: 'true_false'; value: boolean }
  | { type: 'fill_blank'; values: Record<string, string> }
  | { type: 'text'; value: string };
```

**Invariant:** an attempt references `testVersionId`. Editing a published test creates a new version; in-flight attempts keep rendering the version they started with. In the student flow, test content is fetched **only** via `GET /app/attempts/:id`.

---

### 7.1 Word milestone content contract (foundation)

Every question write and publication uses the same interaction invariants:
nonempty prompt and option text, supported type, choice/key cardinality,
nonempty accepted blank answers, unique blank ordinals matching the prompt,
media/audio consistency and content projection validity. Single-choice and
true/false require exactly one correct option; true/false has exactly two options.
Points must be greater than zero, no greater than 999999.99 and have no more than
two decimal places. Do not silently round source or teacher-entered points.
Publication requires a nonempty exam and an exact total within numeric(8,2).

Teacher question metadata is nullable: level is `pre_a1`, `a1`, `a2`, `b1`,
`b2`, `c1` or `c2`; skill is grammar, vocabulary, reading, listening, writing
or speaking. Bank and shared-group authoring requests cap choice arrays at eight;
existing stored arrays remain readable, copyable, publishable and restorable.
The stored editable contract is separate from the capped request contract.
Import assessment also refuses an included question above the authoring cap;
excluded questions do not prevent commit. Publishing freezes both metadata
fields, including grouped questions, and restoring a version preserves them.
Draft test-list skills are distinct, sorted and omit null values.

Question-bank filtering is OR within level or skill and AND across dimensions.
`tagMatch=any|all` preserves today's any default. Each metadata facet omits only
its own dimension while retaining the other filters and normal owner scope;
tag enumeration omits tag filtering. These reads do not promise a new atomic
cross-query snapshot or measured performance.

`ContentDocument` is an application-owned discriminated union: `legacy_markdown_v1`
retains the exact historical Markdown string; `semantic_v1` contains typed
paragraphs, three heading levels, lists, tables, asset references and inline
text/marks, safe links, line breaks and stable gaps. Its normative structural
schema is in `api/openapi.yaml`; aggregate and cross-node rules are described in
[the content contract](plan/19-content-contract.md) and enforced by both domain
validators. Unknown fields, answer metadata, raw HTML nodes and editor-specific
JSON are refused. Text is not rewritten or Unicode-normalized during validation.

The general content components and value object provide the foundation for the
inline option and question prose profiles below. Each new profile needs matching
persistence, publication, restoration and delivery before enabling its writes.
Media/group integration is pending.
The renderer never resolves an asset ID
without an authorized media binding. Existing question payloads above remain the
active contract with the additive option and prose rollouts below.

**Inline option rollout (W-05a).** Choice options may carry optional `content`
using the `OptionContent` subset: exactly one semantic paragraph of text marks
and line breaks. Existing option text is literal, not Markdown, and stays exact.
`text` is the deterministic plain projection when `content` is present. Preserve
both fields in bank copies, version snapshots, restored drafts, previews,
attempts, results and teacher review. The document cannot contain links, gaps,
assets, source metadata or answer keys; grading remains in normalized columns.
An omitted document on update may retain existing formatting only when the
current option ID and text match. Stale or changed legacy writes fail atomically;
explicit `content: null` removes formatting. Options are plain inputs, as the
deck draws them; `VITE_RICH_OPTION_EDITOR=true` (off in production)
enables the pilot affordance to format plain options; readers and editing
existing rich options remain available. This is not an import release or final
editor acceptance. Grouped content and shared audio remain on their existing paths until their
separate integration gates pass.

**Question prose rollout (W-05b).** Since R4 (v0.10.0) a prompt and an
explanation are written in the rich editor by every teacher, in the builder, the
bank's question editor, the group composer and the import review; the opt-in
`VITE_RICH_QUESTION_EDITOR` is gone. Optional `promptContent` and
`explanationContent` support semantic paragraphs, headings, lists and tables with
text marks, breaks and safe links. `QuestionContent` excludes assets and gaps;
`QuestionPromptContent` additionally allows bound gaps for fill-blank questions. `prompt`/`explanation`
are exact plain projections when their document exists; otherwise they retain
legacy Markdown semantics. Rich fill-blank prompts require a bijection between gap node IDs and blank
`gapId` values. Labels never bind answers. Legacy `{{n}}` prompts retain ordinal
binding; rich explanations support all five types.
Snapshots, restoration, bank duplication and all relevant readers preserve these
fields. Active attempts and learner previews never contain explanations; results
release both explanation fields only when `review.showExplanations` permits it.
Legacy writes omitting a rich field must leave its companion string unchanged;
otherwise the update fails atomically. Explicit null clears a document. A
field is written in the form it is stored in: a document in the rich editor, a
Markdown string in the Markdown editor; a field with no text is rich text, and
so is the builder's and the group composer's starter prompt, which opens empty
under the placeholder while its text is still the starter. The
form changes only through "Switch to Markdown" (which keeps text, bold, italic,
strikethrough, headings, lists, tables and links, and drops underline,
superscript and subscript) and "Apply conversion", each after its confirmation.
"Markdown" is unavailable while a fill-in-the-blank prompt holds a gap, because
the gaps bind the accepted answers. Markdown, for the teacher and the student,
reads GitHub's tables and `~~strikethrough~~` and no other GFM syntax.
Conversion from Markdown is explicit, validates the supported subset
and refuses unsupported structures (for example a table's column alignment,
raw HTML, code, images, quotations and links that are not HTTPS) without
changing the original.

Formatted clipboard content follows the same principle: parse locally into the
allowlisted semantic vocabulary, preview the complete resulting field, and apply
only after confirmation. Preserve supported marks, list starts, table spans and
safe links; adapt fonts, colors and spacing to the application's design. Reject
files, active/hidden content, unbound gaps, unsupported styles or incomplete
structure as one paste. The one exception is an image inside copied content
(`img`, `picture`, `svg`, `video`, `canvas`): it is left out without being
loaded, and the preview says how many were left out; a paste holding nothing
else is refused like a file. No fallback to text without the teacher explicitly using
plain-text paste. The converter cannot infer answer keys from visual formatting.
Cancel/stale preview leaves current edits unchanged; one undo reverses insertion.
Field profiles and aggregate budgets apply to the whole resulting document.

### 7.2 Word milestone group ordering (approved, not yet enabled)

For the new group-aware delivery version, `shuffleQuestions` shuffles complete
question groups and standalone questions as units within each section. Sections
retain their authored order. Members of a shared passage/listening/cloze group
retain their authored order even when the groups move. With shuffle disabled,
all units retain authored order. These rules were approved by Thuong as D-03.

Snapshot group membership and member order at publication. Persist the delivery
algorithm version with the assigned snapshot/attempt so reload, takeover and
restoration cannot choose a new deal; preserve stable answer IDs. Historical
versions/attempts continue using their existing section-scoped algorithm and
seeds. `test_versions.delivery_version` stores this marker: existing standalone
versions default to `section_v1`; prerelease group snapshots are classified as
`group_v1` without changing their content or identities. New publications
explicitly write `group_v1`, whose standalone ordering matches `section_v1`.
Rollback refuses to remove the marker while group-aware snapshots exist.
A choice member with semantic references to its option labels declares
`optionOrder: fixed`; assignment with option shuffling is rejected for that member.
Other members use `shuffle`; this never changes the member order inside a group.

### 7.3 Word milestone group graph (contract foundation, not yet enabled)

`QuestionGroup` owns ordered members and materials, optional rich instructions,
and explicit recording bindings. A material's gap targets either a choice member
or the stable `blankGapId` of a rich fill-blank member. Printed labels and mutable
answer-row IDs cannot bind responses. Each material gap has exactly one target;
a response appears only once across that group's material gaps. Gap names are
local to their material or question. Empty groups are valid drafts but cannot
publish; existing sections are not reinterpreted as groups.

The complete resolved context is bounded to 200 members, 16 materials, 16 shared
recordings and 4 MiB, with per-document content limits still enforced. Materials
may repeat an audio asset but resolve it to one recording binding in the group;
the same asset cannot also grant a per-question allowance inside that group.
Distinct member recordings can keep their existing per-question policy. Asset
kind, existence, authorization and deletion locks remain required at persistence.

Copying remaps group, question, material, answer, gap and recording identities,
preserving all grading data, labels, order and content. Immutable media IDs are
reused through new protected bindings. The teacher-only group API and independent
bank editor and mixed builder are available on the milestone branch; release
still requires import and pilot acceptance gates. Relational ownership uses a nullable section owner for each group
(null means an independent bank group) and explicit ownership/order on its member questions.
Section units distinguish standalone questions from groups. Material gaps and
media references have relational bindings; cross-group response/playback links
are rejected. Owned children cannot be archived separately. Restrictive owner
foreign keys require explicit whole-graph cleanup; existing section deletion
cannot leave questions detached from their context. Internal graph creation and
copy materialization are atomic with their audits. Legacy question operations
hide or refuse owned children; active and archived groups protect referenced
media under the same asset lock used by deletion. Full-group editing checks an
aggregate revision; section groups also check the enclosing test revision. Edits
replace content and member order atomically. Bank archive/restore/delete checks
the same revision; permanent deletion requires archival and keeps audit history.
Removing a section-owned group deletes its complete draft graph and compacts unit
order. Copy materialization checks an observed source revision and creates new
editable identities. Draft/snapshot/delivery readers preserve the complete graph.
The builder inserts bank groups as complete independent copies into an explicitly
chosen section, and can save the current group graph as an independent bank copy.

A question crosses a group's boundary without leaving its test's draft. A standalone
question dragged into a section's group, or moved there from its menu, becomes a
member copy: the group gets a new question with new option and blank ids and shuffled
options, and the section's reference to the bank question is dropped. The bank
question is not touched, because other drafts may still reference it; the builder says
so after the drop. A member dragged out of its group becomes a unit of the section at
the drop position. Normally it keeps its id and becomes an ordinary question of the
bank, owned by the test's owner and so listed, editable and protected by the usual
reference check. A member that is an unedited copy of a bank question that is still in
the bank goes back as that original instead: the copy remembers the question it was
made from, it is removed, and the original takes the drop position, so a round trip
leaves one bank question, not two. A copy edited since stays a new bank question, and
the builder says so; one whose original is deleted, belongs to another teacher or is in
the test already is not restored. A member also moves to another group of the same
test, or to another place in its own, in place and under its own id, and keeps the
memory of its original. A member that a gap of its group's shared
material is bound to does not leave its group until the teacher unbinds it in Shared
content. A group left with no member stays as an empty draft and cannot publish. A
bank group is neither a source nor a destination. One transaction does the move under
the test's revision and the revision of every group it touches, so a stale tab meets
`STALE_WRITE`; it is not optimistic, and the builder goes on from the revisions in the
answer. No published version, deal or attempt is touched. The next publish freezes the
new structure, and a version diff against the draft reads a copy into a group as one
question removed and one added, not as a move.

Draft totals and tag filters include owned members alongside standalone questions.
Listening counts count each question once when it has its own audio or a shared
group recording. Independent bank groups are not part of a test's totals. The
legacy question-only whole-outline writer refuses a draft containing groups with
`GROUP_OUTLINE_REQUIRED` before changing metadata or structure; metadata-only
updates remain supported. The group-aware writer accepts complete mixed units and
moves whole groups transactionally. The builder keeps new section client identities
until server IDs arrive, preserving edits and order during acknowledgement. The
outline distinguishes group and standalone IDs, moves empty groups as units and
keeps continuous learner numbering across member questions. Group and outline
writes share a serialized enclosing-test revision. Only acknowledged own moves
advance the active group's revision baseline; external conflicts remain visible.
Preview and publication flush pending content and outline writes. Group findings
open the owning group editor instead of the standalone question endpoint.

The independent group bank supports search, remembered URL filters, newest-first
updates, inline/menu duplication and bulk archive/restore/permanent deletion.
Copies keep the teacher on the list with a small indicator. The group editor has
one active rich editor, ordered materials/members, explicit gap-target selectors,
authorized image/audio picking and upload, HTTPS links and shared recording policy.
Historical Markdown material remains editable. Material preview preserves undo
history; full desktop/phone preview uses the learner renderer without answer keys,
explanations or transcripts. Archived groups must be restored before editing.

Full-group autosave serializes writes with the observed revision, keeps newer
keystrokes when an earlier write is acknowledged, and flushes before route exit.
Incomplete edits remain locally recoverable. Quota/storage errors are visible;
only a server acknowledgement marks the matching revision saved. Recovery from a
stale revision never overwrites current data: the teacher can edit the recovered
content and save a fully remapped independent bank copy.

Publication freezes the entire group into version-owned rows, including ordered
units, member order, materials, stable gap targets and explicit recording policy.
Frozen context points only to frozen question/blank identities and immutable media;
editing or deleting a source group cannot alter it. The app role cannot update
these new snapshot graph rows. Deleting an unreferenced version removes its owned
graph together. Restoring a version replaces the current owned draft graph with
fresh editable copies and remaps both ends of material/blank gap links. Duplicating
a draft preserves mixed unit order and independently copies every group; standalone
bank references retain their existing duplication semantics. Permanent test deletion
removes its owned draft groups after the assigned-version reference checks pass.
Published previews resolve the selected default when no version is specified,
read one coherent frozen version, and expose ordered sections/groups plus safe
material bindings. Their projection never selects answer keys or transcripts;
media URLs come only from relational bindings on that version. Group material
appears before its first member, and material gaps link to the corresponding
question. Teacher playback consumes no student allowance. The builder's preview,
Test detail and the import's "Preview and create" draw the take-test engine in a
frame, Computer or a 390px phone, laid out by the frame's width rather than the
window's (T-D4.16, DG-188): the paper can be answered, and what is chosen is kept
only while the preview is open; nothing is saved, recorded for integrity or
counted as a play, no time is shown, because a time limit is the assignment's,
and Finish says that students submit there. The question group's preview dialog
and Assignment detail keep the read-only paper.
Existing version questions and historical attempts are unchanged.

Archived tests must be restored before creating a draft from a version or changing
their default version. Both flat and grouped tests return `TEST_ARCHIVED` without
rewriting content. An unused, non-current version may still be deleted while its
parent is archived; existing reference checks continue protecting assigned history.

Start, resume and read-attempt responses include the same safe frozen group
projection. Membership comes from the attempt's version, regardless of the test's
current default. The tests module owns the reader, invoked through an application
port after attempt authorization. Shared media is reachable only through protected
relational bindings on a version the learner has an attempt on; assignment
targeting alone grants no media access. A missing context reader fails explicitly
instead of serving grouped questions without their materials. Learner material
is drawn by the renderer the teacher's preview shares: in a passage pane beside
the question from 768px, and below 768px in a drawer from the left that lies
over the question and opens from a handle at the screen's edge or by a swipe
(§9). A shared recording's card is drawn at its audio node in the stimulus pane,
in reading order, the drawer below 768px; with several recordings each card is
headed by its node's label, and a recording no node names is drawn after the
materials. The pane stays
mounted while the student moves between the group's questions and across 768px,
so a recording plays on behind a closed drawer. Stable gap targets navigate to the
question or blank input without losing pending answers. Group players stay mounted
across child navigation.

Learner results retain the frozen material when filtering questions and expose
only transcripts released by each recording's `showTranscriptAfterSubmit` flag.
The score, answer-key and explanation flags do not override that release policy.
Teacher paper review includes the complete shared transcript and that attempt's
recording counts. Grading by question includes only the selected question's
group, without an aggregate playback count across students. Material links focus
the matching question where navigation is available. Review playback is unlimited
and does not append attempt playback events or change recorded counts. Legacy
flat results/reviews keep their existing payload and require no group reader.


---

## 8. Screens & routes — teacher (`/teacher/*`)

`TeacherLayout` (R4, `layouts/TeacherLayout.tsx` and `layouts/shell/`): a sidebar, a top bar with the breadcrumb trail, the search, the bell and the theme toggle, and the page. It follows the theme. A route declares `handle: { crumb, width?, sidebar? }`: the whole breadcrumb trail, whose last crumb titles the document; `width: "full"`, the page frame's one exception (the import review); and a collapsed start for the visit (the import review asks for one). Every other route renders in the one page frame of §12.

- **The sidebar** groups the destinations as the deck does (Home; Teaching: Assignments, Grading, Classes, Students; Content: Tests, Question bank, Media), each shown only to a user with the permission its list operation declares. From 768px it is in the page's flow, 248px expanded or 60px collapsed (icons, the label as the name, no badge), as the user last chose; the choice is kept in `localStorage` under `quizzivy.sidebar` and the sidebar never collapses by itself. Below 768px it is a 272px drawer over the page that traps focus and closes on every navigation.
- **Layout by content width, not viewport.** `<main>` is the scroller; a page lays itself out by the width of its content area (`useContentWidthAtLeast`, `useContentBand`), so collapsing the sidebar can widen a page into its next layout. The only viewport breakpoint is the shell's 768.
- **One outlet at every width**, so a page keeps its state across 768. A route change returns `<main>` to the top and moves focus to it; a change of a parameter alone does neither.

| Route | Screen | Key behaviour |
|---|---|---|
| `/teacher` | Dashboard | Figures, submissions over 7, 14 or 30 days, today's openings and closings, recent activity with the Flagged marker, live taking counts; the teacher home API below. Polls while the user is active. |
| `/teacher/tests` | Tests list | Cards: title, status, skills, #questions, total points, updated. Filter by status. Create / duplicate / archive; permanent delete for unreferenced archived tests. A new test opens in the builder. |
| `/teacher/tests/:id/edit` | Test builder | Left: outline with drag-to-reorder. A question also drags into a shared-context group, out of one and between two (§7.3): each member row has a grip, the group's header takes a drop while it is tinted, and every move has a menu entry and a keyboard path. Right: question editor incl. **question media** (§11.1). Autosave debounced 1.5s. **Publish** validates: `points > 0`; choice questions have ≥1 correct option; `fill_blank` has ≥1 accepted answer per blank; audio questions have a processed asset; no empty sections. Publishing takes an optional change note. |
| `/teacher/tests/:id` | Test detail | Student-eye preview of any version, with its history, the unpublished changes and the diff between versions. Restore a snapshot into a draft, select the default for future assignments, or delete an unused non-default version. |
| `/teacher/imports`, `/teacher/imports/new`, `/teacher/imports/:id`, `…/review`, `…/confirm` | Word import | History; one workspace per import for upload or paste and processing, its Sources pane beside its status pane (DG-207); the review workspace; preview and create (§16, `docs/plan/18-word-import-ux.md`). Shown only while `GET /teacher/imports/capabilities` reports intake enabled. The Sources pane says what leaves Quizzivy when an import is read with AI (§16). |
| `/teacher/question-bank`, `/teacher/question-bank/new`, `/teacher/question-bank/:id` | Question bank and editor | Type, level, skill and tag filters + full-text search. CRUD. Media badge + inline preview. |
| `/teacher/question-bank/groups`, `/teacher/question-bank/groups/:id` | Group bank/editor | Independent complete-group copies, materials and member editing, shared recording policies, revision-safe autosave and account-scoped local recovery. |
| `/teacher/media` | Media library | Uploaded audio/images: name, duration, size, where used. Rename, replace with the same kind, set an audio file's default play limit (§11.1). Delete blocked if referenced by any published version. |
| `/teacher/assignments` | Assignments list | Test, targets, window, status, `submitted/total`, flagged count. |
| `/teacher/assignments/new`, `/teacher/assignments/:id/edit` | Assignment wizard | Test, students, schedule, rules: duration, attempts, shuffle, review options, integrity policy (§10.3) and the note to students. Starts from the teacher's assignment defaults. |
| `/teacher/assignments/:id` | Assignment detail | Stats and the Students, Questions and Settings tabs. The roster is per student: not started / in progress (live remaining, live focus-loss) / submitted / graded; it polls every 15s while the assignment is `open` and the user is active. Extend, per-student overrides, reset, void, duplicate, reopen and close early, each with confirm. The v0.9.x `…/attempts` address opens the Students tab. |
| `/teacher/attempts/:id` | Review & grading | Per question; auto-graded shown; `short_answer` gets points input + comment + **sample answer panel**. Audio questions show plays used vs allowed. **Integrity tab** (§10.4). "Finish grading" → `graded`. |
| `/teacher/grading` | Grading queue | Unmarked manual answers across the caller's assignments, by student or by question (below). `teaching.grading`. |
| `/teacher/students` | Students | Table with search, class and account (Google or password) filters; create into one's own classes; edit; reset one password or up to 40 (§5.4). |
| `/teacher/classes`, `/teacher/classes/:id` | Classes | Cards with their codes; CRUD, schedule and room, members, **join-code card** (§6.4). |
| `/teacher/settings/:section?` | Settings | Profile, Sign-in & security (password, Google, signed-in devices), Notifications, Assignment defaults, Appearance, AI assistant (`ai.use`) and the API reference for its holders. |

**The AI assistant's surfaces** (D19 to D22; V-06 to V-14 of the v2 deck, `docs/plan/74d-d4-deck-update.md` section G). Every row is built: the first three in T-D4.34, the question editor's in T-D4.35, the grading card in T-D4.36, Assignment detail › Questions in T-D4.37 and the import workspace's in T-D4.39; Settings › AI assistant is T-D4.38's. The operations behind every one are built (§15). A panel is a dialog, so none adds a route.

Every entry point is drawn only while `GET /public/status` says `ai` and the caller holds `ai.use`. For a teacher who is paused, or whose allowance or the centre's pool is used up, it is drawn disabled with the reason, not hidden (§12).

| Where | Entry point | Operation |
|---|---|---|
| Test builder, title bar | "AI check" and "Generate" before Preview; both flush pending edits first, as Preview does | `checkTestWithAi`, `generateAiQuestions` |
| Question bank, header | "Clean up" and "Generate" | `cleanUpQuestionBank`, `generateAiQuestions` |
| Test detail | "Make a variant", disabled without a published version | `makeTestVariant`, then `duplicateTest`, `duplicateQuestion` and `updateQuestion` |
| Question editor, in the bank and in the builder | "Write with AI" on the Explanation; "Suggest wrong options" on a single or multiple choice question | `writeAiExplanation`, `suggestAiDistractors` |
| Grading card | "Suggest a score" on a short answer while "Let AI read student answers" is on; with it off, one line that says so and links to Settings › AI assistant | `suggestAiScore` |
| Assignment detail › Questions | "Explain results with AI", under the same switch (with it off, the same kind of line); "Make a practice set" opens Generate and saves a new draft test "{title} · Practice" | `explainAssignmentResults`, then `createQuestion`, `createTest` and `updateTest` |
| Import workspace, Sources pane | the "Read with AI" card, which mirrors the preference "Read imports with AI" | `createWordImport` with `readWithAi` |

The builder's Generate adds the kept questions where its source belongs: from a passage that a shared-context group holds, to that group, as new members after its existing ones, through the group's own update under its revision ("Add {n} to {group}"); from pasted text, a topic or nothing, to the open question's section as new questions ("Add {n} to {section}"). Either way the first new question opens.

A variant is a new draft test named "{title} · Variant", never a version of the open one. A copied draft names the same bank questions as the test it copies, so each changed question is duplicated and the change is written to the duplicate: the test and its questions are untouched. A variant covers the standalone questions; the members of a group are not varied and are copied as they are. The bank clean-up merges nothing: a near-duplicate offers "Open" for each question and "Keep both", and tag suggestions apply through the bulk tag write (DG-197).

**Settings › AI assistant** (`/teacher/settings/ai`, `ai.use`) holds the month's usage from `getAiUsage` (used of the allowance, what is left, a bar and the five rows generation, grading, explanations, imports and checks, with the date the allowance resets, the first of the next month in `APP_TIMEZONE`: "It resets on 1 November"), the preferences (`UserPreferences.ai`: the language the assistant writes in, tagging what it drafts as "AI draft", letting it read student answers, reading imports with it), and "Ask for more credits" (1,000 to 100,000 and a reason of at most 500 characters) while the centre's rule is to take requests and none is waiting, and the request waiting, with its date, until an Admin decides it (DG-200). With the assistant off the section says "AI is not turned on for this school." (DG-201).

**Admin › AI credits** is the pool, the requests waiting, the teachers with their month and the rules. *Availability: R5* (T-R5.35, in the Admin console R5 builds; none exists before). D4 ships its operations (§15) and `cmd/maintenance ai-credits` (`docs/setup/operations.md`), which the owner uses until then. The rule for a pool that runs out has one value, pause everyone; "Add credits" is the Admin's approval, and the product bills nobody (DG-196).

No student screen mentions the assistant. The Test intro's sentence about cloud processing waits for Thuong's wording (§17).

The teacher home API (`getDashboard`) accepts `range=7d|14d|30d`, defaulting
to `14d`. Only `submissions` depends on this range: one zero-padded calendar day
per entry in the actor's zone, oldest first with today last, counting `submitted`,
`timed_out` and `graded` papers by `submitted_at`. The total is their sum. The
integer mean percentage rounds half up over graded papers with a valid score;
graded papers without scores still count, and the mean is null when none has a
score. The existing nullable score columns and historical rows are unchanged.

`takingNow` counts distinct students and assignments with an `in_progress` paper
whose deadline is strictly in the future. `today` holds up to 20 published
assignment openings and effective closings in the actor's calendar day, earliest
first; an early close uses the earlier of `closed_at` and `closes_at`.
`notSubmitted` counts distinct enabled targets the caller reaches without a
handed-in paper; a former target's submission does not subtract from that roster.
`recentActivity` holds the ten newest events: one state per non-voided attempt
(started while in progress, submitted once handed in), and student-like joins
through a taught class's code. Attempt flags carry through; joins are unflagged.
These new readings contain no score, band or answer.

`getTeacherSummary` supplies live assignments matching the caller's open list,
unmarked manual answers in reachable handed-in papers, and the caller's unread
notifications. The grading count is null and its query is not run without
`teaching.grading`; live assignments is currently always a number. Both home
operations use `.Own()` even for an Admin, while `listAttempts` keeps its wider
scope. The application supplies the new queries' clock and resolves the IANA
calendar zone through `ports.Zones`; T-R4.7 wires the identity effective-zone
query. Only a stored NULL defaults to `Asia/Ho_Chi_Minh`; an invalid stored zone
or ineligible account propagates an error. A nil port retains the default for
existing callers. Frontend date displays react to the caller's accepted display
zone; existing local datetime inputs and their rules preview remain fixed-HCM
until T-R4.43. Legacy readings retain their
SQL clocks. An absent notifications summary port returns 501.

The grading queue requires `teaching.grading` and preserves the existing assignment
and paper scope. It includes saved, unmarked manual answers in submitted or timed-out
papers; a manual score of zero is marked. Student groups use student IDs; question
groups use the assignment and frozen question IDs together. Counts cover the complete
filtered set before the deterministic first200 items, with oldest submissions first
and null times last. Optional assignment and student filters apply to counts and items.
Group labels use current names or the global one-based frozen question number;
assignment titles are current, while question keys, rich content and group context
come from the published version. The aggregate, groups and item prefix share one SQL
statement snapshot; later immutable enrichment does not claim a whole-request
repeatable-read snapshot. Reading or exhausting the queue never finishes grading;
the teacher still invokes Finish explicitly. The rebuilt web queue is T-R4.28.

When nothing is left to mark (T-D4.21b) the header reads "Không còn câu trả lời nào cần
chấm." and the page checks for papers that are marked but not finished: every submitted and
timed-out paper the caller reaches with nothing waiting for a mark (`listAttempts`,
`pendingGrading=false`, read to the end), under the queue's filters. "Đã chấm xong tất cả" shows
only after that check completes and finds none; while it runs, or after it fails, the page says only
that no answers are left to mark. The papers it finds are listed under "Sẵn sàng hoàn tất", below
the panes, grouped by assignment with the oldest hand-in first, five to a group before "Show
more". A row's Review opens its check, read fresh from `getAttemptForReview`: the score, the
answers given, whether the teacher marked it, it was marked automatically or it was left blank, and
any flag. Finish grading there is enabled only while that read's `pendingManual` is 0, and the paper
is read again before the request; after a Finish the next paper's check opens with focus on its
Finish. While answers wait the list folds to one line and the header links to it. Nothing finishes
on its own, and there is no Finish for a whole assignment.

Teacher list behaviour (approved change request, 2026-09-22):

- Tests, questions, media, classes, students and assignments support explicit multi-selection and confirmed bulk removal. Class rosters support bulk membership removal. Failed items remain selected with an individual explanation; successful items are not retried. Attempt history, grading records and integrity/audit events remain retained.
- Tests and questions expose duplication beside the row menu and inside it. A duplicate leaves the teacher on the list and marks the source and newly created row until navigation.
- List search, filters and pagination survive navigation to a child and back for the signed-in session. Explicit shared URLs take precedence. The question bank shows updated time and orders newest-created questions first.
- The question bank's usage count expands a nested table of currently attached test outlines, fetched on demand, with links to their builders. Frozen published snapshots are excluded from this count and remain unaffected by bank edits.
- Archive tests/classes or disable student accounts before permanent deletion. Assignment deletion requires a draft or closed assignment without attempts. Foreign-key references to assigned work block deletion. Student actor references in retained audit history also block deletion; the existing manual anonymization policy remains separate.
- Published snapshots are never edited in place. Editing an old version copies its frozen content into new bank questions and replaces the editable draft after confirmation. Existing assignments retain their version. Switching the default does not reset the monotonically increasing publication counter. Only unused, non-default versions can be deleted.
- Empty builder groups accept question drops; double-clicking a group title opens rename. Switching questions flushes the current editor and uses the returned saved record. Fresh starter text disappears on focus without scheduling an invalid empty save. Tags suggest recent and matching existing names.
- Assignment duration offers 30, 45 and 60 minutes plus a custom minute input. Attempt count is a number input defaulting to 1. Focus loss offers unlimited, none allowed, or a custom positive count.

## 9. Screens & routes — student and public

`StudentLayout` is the deck's student shell: a 60px top bar, no sidebar,
safe-area padding, and one route outlet that stays mounted at every width. It
branches at 768px. From 768 the bar holds the destinations beside the logo
(Home, Classes). Below 768 they are a bottom tab bar (Home, Classes, Me), and a
detail screen (the intro, the result, Settings) swaps the logo for a back arrow
and its title and hides the tabs. A destination whose module has not shipped is
absent: Learn, Grades, Messages and the bell in v0.9.0. Home's item carries a
badge, the papers still to do that close within seven days (DG-82). The avatar
button opens the account menu: Settings, the light or dark theme, Sign out.
Each page is one centred column with its own maximum width (§12). The focus
engine has no navigator column and no stored width. `AuthLayout` is the brand frame for sign-in, forgot password, the Google callback, change password and join. The brand panel shows from 900px, except on join and its Joined state, which stay one column at every width. The system pages use `SystemFrame`.

| Route | Layout | Key behaviour |
|---|---|---|
| `/join`, `/join/:code` | Auth (one column) | §6.2: inline preview, then Joined. Every failed lookup shows one plain message, with no hint about which classes exist. `/join/:code/confirm` redirects to `/join/:code`. |
| `/login` | Auth | Password form + "Tiếp tục với Google". One message for an unknown email, a wrong password and a disabled account; a 429 shows the server's message. In a join context the subtitle names the class and a successful sign-in continues the join. |
| `/forgot-password` | Auth | Static help (§5.4); no request. |
| `/set-password` | Auth (one column) | §5.4: the page an emailed link opens. It reads `#token=` from the address, removes it with `history.replaceState` before any request, then asks `checkSetPasswordLink`. Checking is a skeleton; a valid link names the account ("Đặt mật khẩu cho {email}") above a new and a confirm password with the §5.4 rules; an invalid one shows one message, whatever the reason, and a way to the sign-in page; while password sign-in is off it says to sign in with Google. Success enters the session and goes to the account's home. |
| `/app` | Student | Home. A greeting for the time of day and one line about what is next. The resume card for the attempt in progress that closes soonest; "Tiếp tục làm bài" resumes it at once. Coming up: every other paper still to do, each row a link to its intro, with a pill (in progress, due today or tomorrow inside 24 hours, open now, opens …). Recent results, newest first: the score if allowed, "being graded" or "submitted", each a link to its result. No filters. Only the intro starts the clock. A student in no class with nothing assigned is offered the Join dialog (§6.2). |
| `/app/assignments/:id` | Student | Intro: class and title, three facts (time limit, questions, attempts used of allowed), and "Trước khi bắt đầu": **the rules stated plainly**, generated from the stored policy and dates (availability, the timer, fullscreen, copy and paste, leaving, the audio plays per §11.4, what the result shows). "Bắt đầu làm bài" asks "Bắt đầu ngay?" first; "Tiếp tục làm bài" resumes at once. A test not yet open, closed or with no attempts left says so in the button's place. A start the server refuses says why above the button: `409 MAINTENANCE_SCHEDULED`, for a start that would run into a maintenance window, with the server's message. When the teacher wrote a note for the students, it is drawn between "Trước khi bắt đầu" and the button: the teacher's initials, "Ghi chú từ {name}" and the text, as plain text with its line breaks; with no note nothing is drawn. The score sentence says "sau khi nộp bài" for a result released on submit and "sau khi bài đóng" for one released after the close. |
| `/app/attempts/:id` | Focus | The engine (below). §10, §11.3. |
| `/app/attempts/:id/result` | Student | The summary card (the score ring if allowed, the class, the title, one sentence on where grading stands), tiles, then every answer honoring `review.*`, with the transcript if `showTranscriptAfterSubmit`. While answers wait for the teacher the ring shows the score so far and the tiles say what waits. A line names what the policy hides. A result released after the close is withheld until then: the ring is a lock, no score, mark, answer key, explanation or grader's comment is sent, and the line and the summary say when it is released (`releasesAt`). When the server sends `classAverage`, one muted line under the summary sentence reads "Điểm trung bình của lớp: {n}%." Filters All / Wrong / Waiting: Wrong only when scores are shown, Waiting only when something waits; an empty filter says so and offers all questions. The phone header reads "Kết quả". |
| `/app/classes` | Student | The classes joined, as cards: name, description, teacher (with photo), when and where it meets, and "Next", the paper that comes next in that class. A card is not a link, and nothing here leaves a class. One action, "Tham gia lớp", opens the Join dialog (§6.2). |
| `/app/settings/:section?` | Student | Profile (default: name, email read-only, language), Sign-in (password, Google) and Appearance (theme, "Chữ lớn hơn khi làm bài"), under a segmented switcher. Every section stays mounted, so a form survives a change of section or of width. The old slugs redirect: `security` to `sign-in`, `preferences` and any unknown slug to Profile. |

Shared:

- `/change-password` (§5.4).
- `/403`, and the 404 page for any unknown route.
- The unexpected-error page, with a copyable error ID (the server's `requestId` when there is one).
- The maintenance page.

"Home" on these pages is the caller's console, or `/login` when signed out. The system pages (403, the 404 page, the unexpected-error page and the maintenance page) ship in the entry chunk, so they render when a lazy chunk cannot load; `/change-password` is a lazy route.

**Boot splash.** The app starts behind the deck's splash. Before the bundle runs, `index.html` paints the mark, the track with the first step's 12% fill and the version line; the step label stays empty until the bundle writes it.

- It shows three steps, and the bar moves only when a step is done, with no minimum duration:
  1. "Đang kiểm tra phiên đăng nhập…" — refresh and `/auth/me`.
  2. "Đang tải lớp học của bạn…" — the first route's code.
  3. "Sắp xong rồi…" — the hand-over.
- **Slow:** eight seconds without a step shows "Tải lâu hơn bình thường" and "Tải lại".
- **Offline:** a session restore that got no HTTP response shows "Mất kết nối" and retries after a ten-second countdown, or as soon as the browser is back online.

**Overlays** stand over a mounted page, which becomes inert; focus moves into the overlay. They follow the theme over every console.

- **Maintenance** closes when `GET /public/status` reports no window under way. It checks on "Kiểm tra lại", and every minute while the user is active. Closing refetches everything.
- **Sign in again** (§5.2).
- **A newer build.** `/version.json` is checked when the tab becomes visible and every ten minutes while the user is active. A different build shows "Quizzivy vừa được cập nhật" at the next navigation, never on the engine's route. A route whose code cannot load shows it at once.
- **Precedence:** maintenance, then sign in again, then a newer build.

---

**The engine** shows one question at a time, except that a run of a cloze's gaps
is one screen ("A cloze" below).

- **Header.** The ✕ ("Thoát khỏi bài làm"), from 768 the title over the save
  line, the timer, and "Nộp bài". The ✕ asks first, except on a locked paper,
  which it leaves at once and without saving. Leaving saves what is
  unsaved and goes to `/app`; a save that fails keeps the student on the paper.
  Below 768 the header has no save line. A strip under it carries one only when
  a save has failed or the device is offline with an answer unsaved, never for
  a save on its way, and carries the strike count where the assignment sets a
  limit (§10.2). A locked paper (taken over, out of time, ended) says why in a
  bar under the header at every width. Once the server has said the time is
  up, the timer and the Submit dialog's time left read 00:00. A deadline lock
  rearms automatic submission at the device's recorded deadline, whichever
  request was refused and even after a second refusal. Submission can still
  fail and retry. A deadline refusal never replaces a closed or superseded lock.
- **Section head.** Above every question the question column states its
  section's title over its instructions, so a part's instructions are read on
  each of its questions, not only on the first. A paper of one section with no
  instructions has no head, nor has a section with neither a title nor
  instructions. A group's own instructions stay in the stimulus pane, under its
  heading.
- **Panes.** A question whose group has shared content (a material or a
  recording) shows the stimulus pane beside the question pane from 768, each
  scrolling by itself. The question pane is then on the sidebar's surface with
  its column at most 600px wide; alone, or on a phone, it is on the page's
  background at most 720px wide. The stimulus pane leads with the group's title
  as a kicker over the heading of its first material with something to read; a
  material that is only a recording has no heading, and a shared recording's
  card stands at its audio node (§11.3). Below 768 the
  question fills the screen and the shared content is a drawer from the left
  over it, 88% of the width and at most 420px, above a darkened backdrop. A
  handle at the left edge ("Mở ngữ liệu", or "Mở bài nghe" for a recording,
  with a dot while the group's recording plays) or a swipe to the right opens
  it; a swipe back, Esc, the backdrop or its close button closes it and returns
  focus to the handle. Until the student has opened a group's drawer once in
  the attempt, the handle nudges (not under reduced motion) and a tip beside it
  says what is there. A closed drawer stays mounted and inert, so the place in
  the passage and a playing recording survive. Every move to another question
  closes it, and so does a gap tapped inside it, which lands on its question or
  blank; while it is open the keys that answer or flag do nothing. Crossing 768
  with it open shows both panes, with nothing remounted.
- **Navigation.** Under the question pane: Previous, a middle, and Next, which
  is "Hoàn tất" on the last screen. The sections rail ("Các phần"), a 236px
  column before the panes, shows when the engine is at least 900px wide on a
  question with no shared content and at least 1180px wide on one with it, and
  never below 768, so from 900 to 1179px it comes and goes between questions.
  It holds one block per section, the section's title beside "n / m" answered
  over a five-column grid of squares, and the legend at its foot. While it
  shows, the footer's middle is the key hint ("← → để chuyển câu · A–D để chọn
  đáp án"; "để chuyển · A–D để chọn" beside shared content; "để chuyển câu" on
  a question with no options). Otherwise, at every width, the middle is the
  grid button, "Câu 4 · đã làm 3 trên 8" ("4 · 3/8" beside shared content from
  768 and below 420px), which opens the question sheet: a bottom sheet below
  768 and a 560px card from 768, with the same blocks in six columns and the
  legend; its name and the answered total are for a screen reader only. It opens on
  the open question's square, keeps focus inside, closes on Esc, the backdrop
  or a square, and closes when the engine grows past the rail's threshold.
  A square is answered (filled), partly answered (a fill-in with some of its
  blanks typed, which still counts as unanswered) or empty; the open question's
  square is ringed in lime and a flag is an amber dot. There is no strip of
  numbered squares.
- **A cloze.** Two or more consecutive single-choice members of a group, each
  bound by a gap of one material and none with an image or audio of its own, are
  one screen: "Câu 23–27 trên 34 · Điền từ vào đoạn văn · 5 chỗ trống". It
  prints their shared plain prompt once (or each member's own prompt in the gap
  card), a chip per gap, and the open gap's card with its options and its worth;
  a choice opens the next gap with no answer, going round. Previous, Next and
  the arrows step over the whole run. Every gap stays a question with its own
  number, square, flag, points and result row: the squares are tied within a
  row ("Cùng màn hình" in the legend) and all ringed while the run is open. A
  member with media, or a gap left alone, is a question of its own whose line
  says "Điền từ vào đoạn văn · 1 trong 5 chỗ trống".
- **Submitting.** "Nộp bài" in the header and "Hoàn tất" open the Submit
  dialog, the only way a student hands a paper in: answered, flagged, time left, and a
  "Đến câu" chip for each unanswered question. There is no review page.
  Submitting saves what is unsaved first. A submission that fails says so in
  the dialog, which stays open. The submitted screen offers Home and the
  submitted paper.
- **An unknown type.** A question of a type the page has no renderer for shows
  a block that names it and offers a reload, and takes no answer.
- **Local drafts.** Student answers awaiting server confirmation are cached
  locally per student, attempt and session until saved or closed. They may be
  restored before the server deadline after a reload. A draft a closed tab left
  behind is sent under the session that wrote it before the student is given a
  new session by "Tiếp tục làm bài" on Home or the intro. The server saves it
  if that session is still the attempt's. If it is refused with one of the
  attempt's own codes (`SESSION_SUPERSEDED`, `DEADLINE_PASSED`,
  `ATTEMPT_CLOSED`) or as a body that can never be accepted
  (`VALIDATION_FAILED`), it is dropped: a superseded session must not overwrite
  a newer session's answers. On any other failure the draft is kept and the
  attempt is not resumed. Explicit sign-out clears the local cache.
- **Held session.** A tab names the session it holds each time it reads an
  attempt. An in-progress paper taken over elsewhere is answered as
  `superseded`: it stays read-only after a reload or refetch, reads no local
  draft, and removes a draft only if its own session wrote it. It writes
  again only when the student continues the attempt there. A tab that holds
  no session is given the current one. Such a read still replaces the
  attempt's beacon token, and two tabs of one browser can share a session.
- **Keys.** A–E choose an option, on a cloze for the open gap; a sixth has no
  key, because F flags the question. The arrows move between screens, as
  Previous and Next do, and stop at either end. Esc closes a dialog, the sheet
  or the drawer. None acts in a text field or another control
  that owns the key, with Ctrl, Alt or the command key held, while a dialog or
  the question sheet is open, or on a locked paper.

## 10. Integrity monitoring (proctoring-lite)

First-class requirement. Lives in `src/features/integrity/`, consumed by `take-test` (capture) and `attempts` (review).

### 10.1 Signals

Every signal produces an append-only event `{ kind, occurredAt, clientSeq, questionId?, meta? }`.

| Kind | Source | Notes |
|---|---|---|
| `tab_hidden` / `tab_visible` | `document.visibilitychange` | Primary tab-switch signal; pair to compute away-duration. |
| `window_blur` / `window_focus` | `window` blur/focus | Catches alt-tab to another **application** — the Visibility API alone misses this. |
| `fullscreen_enter` / `fullscreen_exit` | Fullscreen API | Only when `requireFullscreen` is on. |
| `copy` / `cut` / `paste` | listeners on `document` | Always recorded; **blocked** only when `blockCopyPaste` is on. |
| `context_menu` | `contextmenu` | Recorded; blocking off by default (it breaks assistive tooling). |
| `network_offline` / `network_online` | `navigator.onLine` + fetch failures | Distinguishes cheating from bad wifi. Matters for fairness. |
| `audio_play` / `audio_ended` / `audio_blocked` / `audio_seek` | player (§11.4) | Gives the teacher listening behaviour. `audio_seek` is a jump the player put back on a recording that may not be skipped. The events only report: a question's play is counted by `POST /app/attempts/:id/audio-play`. |
| `resume` | server-side | Re-entry into an `in_progress` attempt: reload, crash, device change. |
| `session_takeover` | server-side | Attempt opened in another tab/device. |
| `page_hide` | `pagehide` | Best-effort final flush via `navigator.sendBeacon`. |

**Deliberately excluded:** devtools-detection heuristics (window-size deltas, `debugger` timing). They false-positive on zoom, split-screen, and extensions, and are bypassed in seconds. Do not implement them.

**Away-duration over count.** A 2-second blur is a notification; a 90-second blur is a search. Store both endpoints so duration is visible, and only count a strike when an away episode exceeds `minAwayMs` (default 3000ms).

### 10.2 Student-facing behaviour

Announced, visible, never silent.

- The intro page states the active rules in plain Vietnamese before starting, generated from the stored policy (§9). If `requireFullscreen` is on, the intro's click that starts or resumes the attempt is what enters fullscreen (browsers require a gesture): "Bắt đầu" in the "Bắt đầu ngay?" dialog, or the intro's "Tiếp tục làm bài". Home's "Tiếp tục làm bài" (§9) goes straight to the paper and does not ask for fullscreen: the engine opens outside it and shows the fullscreen bar described below. A browser with no fullscreen is told so and takes the test as normal.
- Each counted absence opens the deck's `alertdialog` when the student returns: "Bạn vừa rời trang làm bài", one body and one button, "Quay lại bài làm". The body says where the student stands and what happens next, and only what the server does:
  - within the allowance, "Lần này được tính là lần n trong m lần được phép…";
  - past it under `flag`, that the teacher has been told and the answers are safe (D8);
  - past it under `warn`, the count and that the answers are safe, never naming the teacher;
  - with no absence allowed (`-1`), that leaving is not allowed, with no number;
  - under `auto_submit`, that the test is submitted after the allowance, and at the last one that the next absence submits it;
  - with no limit, that leaving is recorded, once a sitting.

  The body does not state the seconds away or that the timer keeps running: the timer is on screen. The dialog does not close from the backdrop and has no close button; the button and `Esc` acknowledge it.
- A small persistent indicator shows remaining strikes when a limit is set: after the save line from 768, in the strip under the header below it (§9). Past the allowance it reads "Giáo viên đã được báo" under `flag`. `maxFocusLoss = 0` retains the unlimited default; `-1` permits no counted departure; positive values permit that many departures.
- `onLimitExceeded`: `warn` = dialog only; `flag` = attempt marked for the admin, student told; `auto_submit` = immediate submission on exceeding the count, retaining answers for grading and recording the violation. There is no cancellation or extra strike. The final answer/event batch is saved before the server grades and closes. While offline, the attempt is locked locally, pending answers are retained and submission is retried with a visible notice.
- Fullscreen exit shows a bar under the header with a "Quay lại toàn màn hình" button, or one sentence on a browser with no fullscreen. It is a bar, never a dialog. Never trap the student: `Esc` always works and there is always a visible way to leave and submit.
- Once the attempt is submitted on an assignment that requires fullscreen, by the student, the timer or an auto-submit, the engine leaves fullscreen itself. That exit is the app's, so from then on no `fullscreen_enter` or `fullscreen_exit` is recorded, the student's own on the submitted screen included. A paper that is locked without a submit (taken over, already ended) stays as it is.
- Under `blockCopyPaste`, a copy, cut or paste is stopped and recorded, and a danger toast says "Sao chép và dán đã bị tắt trong bài này": one toast, however often.
- Under five minutes the timer takes the danger tones (D8). A polite live region says the time left once when it passes 5:00 and once when it passes 1:00.
- While the maintenance or sign-in-again overlay (§9) covers the engine, no focus change is recorded. An away episode already open when one appears is dropped, so time on an overlay never counts against the student.
- The engine adopts the server's deadline from every autosave (`deadlineAt`). It also adopts it from a timer submit that came too early (`409 DEADLINE_NOT_REACHED`; the server allows 5 s of grace). So a teacher's extension, or a maintenance window's, reaches an open attempt on its next autosave.

### 10.3 Policy defaults (per assignment)

`requireFullscreen: false`, `blockCopyPaste: true`, `maxFocusLoss: 0`, `onLimitExceeded: 'flag'`. Conservative on purpose — the teacher opts into stricter modes per test.

### 10.4 Teacher review

What the teacher sees, by D8 (R4). One tone marks "Flagged" everywhere, the danger tokens (`features/integrity/tones.ts`): the dashboard's figure and its Recent activity marker, the assignment's Flagged count, the roster's row, the attempt sheet and the attempt page, and the flagged notification.

- **The Flagged count** on Assignment detail's progress card (T-D4.22) counts the flagged attempts that are not voided: exactly the rows its filter shows, over the whole roster whatever the search. A flagged attempt that is later voided leaves the count.

- **The roster** (the assignment's Students tab) shows Focus lost as "—" or "{n}×", in danger ink after a flag icon once the attempt is flagged, on a row in the danger's soft tone. Where the column is hidden, the same mark sits under the student's name.
- **The attempt sheet** draws a compact timeline from the start (info) to "Submitted" (success, once handed in). Leaving the test and an automatic submission are danger, with the time away on the label ("Left fullscreen · 14 s"); every other kind keeps a neutral dot.
- **The rules** keep §10.5's honest limits beside the integrity policy in the assignment wizard.

`/teacher/attempts/:id` → **Integrity** tab: ordered timeline with event kind, wall-clock time, offset from attempt start, duration for paired events, and the question on screen. Summary strip: total away-time, away episodes ≥ `minAwayMs`, paste count, resume count, audio replays. Neutral text — no red banners, no "CHEATING DETECTED". The teacher judges; the app reports. An audio play with no recorded end shows no duration and is not called ongoing.

Sessions are ordered by their earliest `receivedAt`, with `sessionId` breaking
ties. Within a session, `resume` comes first and `session_takeover` last.
Within each priority, two events with different non-null `clientSeq` values
compare by `clientSeq`; otherwise by `occurredAt` and then event id. This order
is not a guarantee of wall-clock chronology across tabs or sessions.

### 10.5 Honest limits — say this in the UI help text

Browser monitoring detects *this tab* losing focus. It cannot see a second device, a phone beside the laptop, a person in the room, or a printed sheet. The signals are **evidence for a conversation**, not proof. Do not build auto-zero or auto-ban features that assume otherwise.

### 10.6 Client implementation

- One `useIntegrityMonitor` hook owns every DOM listener that records a
  signal or stops one, registered and torn down in a single `useEffect`.
  Four signals are recorded outside it, by a question's audio player through
  `recordAudioEvent` (§11.4): `audio_play` from its play callback,
  `audio_ended` when playback reaches the end, `audio_blocked` when the browser
  refuses to start it, and `audio_seek` when the player puts a jump back.
  None changes a play count. A shared recording's play is written by the
  server (§11.5); the browser records its end and block with `scope: group`
  and `recordingId` in `meta`, without a question id. A known end duration is
  elapsed device wall time since the latest start, clamped to zero if the
  clock moved backwards; it is not total listening time. A resume overwrites
  that start; an end or block clears it. The timeline pairs the latest open
  play once, by question id first or otherwise by recording id. It accepts
  numeric `durationMs` from 0 through 86,400,000, truncating fractions to
  whole milliseconds; missing, malformed or out-of-range durations fall back to the
  nonnegative span between event clocks. Delayed play counts or clock skew
  can leave events unpaired. Overlapping sessions have no play-id matching.
  Other hooks listen to some of the same events and record and stop nothing:
  `useClipboardNotice` has its own `copy`, `cut` and `paste` listeners on
  `document`, only to show §10.2's toast; `useSaveStatus` listens to `online`
  and `offline` on `window`, only for the save line (§9); `useVersionWatch`
  listens to `visibilitychange` on `document`, only to look for a newer build
  (§9). The monitor records and stops whether or not they run.
- Events buffer in memory + `sessionStorage`, flush with the autosave batch, and immediately on `pagehide` via `sendBeacon`. Leaving the engine while the attempt goes on sends what is buffered through the same beacon and keeps the sequence in `sessionStorage`, so a return in the same session numbers on from it; the buffer is forgotten once the attempt has ended.
- `clientSeq` uses an attempt-relative millisecond offset, calibrated by
  `getAttempt` from `startedAt` and `serverTime`, and is never below the tab's
  stored next sequence. It increases within a tab. The offset component caps
  at 2,000,000,000; subsequent local increments can exceed that value.
  Response latency, clock changes and local increments can cause reordering
  or collisions across tabs, even for events in different milliseconds.
  Cross-tab chronology and uniqueness are not guaranteed. Older builds still
  send counters from zero, which the server accepts. The append-only dedup
  key remains `(attempt, session, clientSeq)`; a repeated key is treated as a
  retry and not stored.
- Failed background event flushes do not block answering or manual submission.
  The immediate `auto_submit` policy retries its final answer/event batch before
  confirming submission so the violation is retained with the answers.
- An event transport failure alone does not block input. The explicit `auto_submit` policy locks further answering once exceeded, while preserving and retrying the pending answers.

---

## 11. Audio / listening questions

### 11.1 Admin: upload and attach

- In the question editor, an audio question has: file upload, transcript textarea (admin-authored, optional), and the `AudioPolicy` controls (`maxPlays` default 2, `allowSeek` default **false**, `showTranscriptAfterSubmit` default true, `allowPause` default **true**).
- `allowPause` is "Students can pause" (T-D4.13, DG-183). It is a rule a student is meant to read, not a key, and it applies to a question's audio and to a group's recording alike. A create that leaves it out stores true. **An update that leaves it out keeps the stored value**, for a question by its id, for a member question of a group by its id in the bundle and for a recording by its id; an audio policy that is new in the request (the question had none, or an image) and a recording under a new id are creates and read true. The rule exists because `updateQuestion` and `updateQuestionGroup` replace the whole object, and a tab loaded before this release, or a recovery draft saved on the device, sends none. Publish freezes the value in both snapshots; restoring a version as a draft, duplicating a test and copying a group carry it; the teacher's preview, the student's paper and the teacher's review read it. A question or version from before the control reads true.
- Accepted: `audio/mpeg` (.mp3) and `audio/mp4` / `audio/aac` (.m4a). **Reject everything else** with a plain message. These two cover every current browser without transcoding; supporting `.ogg`/`.wav`/`.webm` means either transcoding or a Safari support matrix, and neither is worth it in v1.
- Limits: audio 50 MB and 5 minutes; an image (PNG, JPEG or WebP) 10 MB (DG-63). Validate **server-side** by sniffing magic bytes and probing duration — never trust the `Content-Type` header or the file extension. The file is streamed to disk, never held in memory, and an image's size in pixels is read from its header.
- A teacher's library holds at most 5 GB unless the deployment sets another quota (`MEDIA_OWNER_QUOTA_MIB`). An upload that would take it past the quota is refused with `MEDIA_QUOTA_EXCEEDED`. A file that is deleted or replaced leaves the library at once and counts toward the quota only while something still names it: a published version, another owner's content, or a question that attaches it outside any group. `GET /teacher/media` reports those bytes as `usage.heldBytes`, so a replacement needs room for the new file and, while a published version holds the old one, for the old one too. The files nothing names are `usage.pendingBytes`, and count toward nothing (§11.2).
- In the library a file can be renamed, and audio can be given a default play limit: unlimited, or 1 to 3. The stored file and its original filename never change. The limit is stored with the file for the editor to start a question's own limit from; it changes no question by itself.
- Upload goes **through the Go backend** in v1 (low volume, and the backend must validate anyway). Presigned direct-to-R2 upload is the P1 optimisation; design the API so switching does not change the client contract beyond the upload call.
- Assets are **immutable**. Re-uploading creates a new `media_assets` row; it never overwrites an existing key. This is what lets `test_version_questions` reference an asset without copying the file.
- A library file can be replaced only by the same kind (`415 MEDIA_KIND_MISMATCH` otherwise). Replacement creates a new
  immutable asset owned by the old file's owner, with its current display name and default
  play limit. In one database transaction, the retained current graph of live editable
  questions and bank or draft-owned group media of that owner points to the new asset;
  each changed group gains one revision.
  Published versions, import review drafts and other owners' bindings retain the old
  readable asset. The response counts distinct changed questions/groups and other owners'
  live editable questions/groups left on the old asset; historical versions and imports
  are excluded. Late same-owner bindings may retain the old asset and are excluded from
  the left counts. The old file leaves the library, and its owner's quota once nothing names it, but remains available to existing
  bindings and later publication/import materialization. The answer's `previous` names the published versions that still
  use it, a URL to download it and the time its object goes. An internal error can follow a
  commit, or its outcome can be unknown; both retain the new object. A retry against the
  replaced old library ID can answer 404. Replacement does not promise idempotent retries
  or an atomic transaction with object storage.
- Client-side pre-check before upload: read duration via an `<audio>` element and reject early, so a student's teacher does not wait 50 MB to be told no.
- **A question's media is one audio file or one image**, attached from the library ("From Media") or uploaded in the question editor, which shows the limits above before it uploads. The audio policy and the transcript exist only for audio. An image carries optional **alt text** of 1 to 1000 characters, `mediaAlt`, that the editor asks for as a description for screen readers. It belongs to the question and not to the file, so two questions that share one image describe it in their own words, and a file replacement (which never changes the kind) keeps it. A write replaces the stored value, so an update that leaves it out clears it. The server refuses a value for a question whose media is not an image with `400 VALIDATION_FAILED` naming `mediaAlt`, and clears it when the media is removed or changed to audio. Publish copies it into the version with the rest of the question, so a later edit of the bank question never reaches a version; restoring a version as a draft and duplicating a group carry it, and the version diff counts a changed alt text as a change to the question's media. Students receive it as `mediaAlt` beside `media` in the paper, the session and the result. It is a description of the picture, not part of the answer key, and the engine, the result and the builder's preview draw the image with it as `alt` or, when there is none, a generic label.

### 11.2 Storage and delivery

- Bucket `quizzivy-media`, key `audio/{asset_id}.{ext}`. Bucket is **private**; no public listing, no public read.
- Served via **short-lived signed URLs** (10 min), minted per request by the backend. The frontend treats `MediaAsset.url` as expiring and refetches on `403`.
- `Cache-Control: private, max-age=600` — long enough to survive a replay, short enough that the signed URL does not outlive its cache entry.
- **A file that left the library is kept, then removed (F-68).** The object of a deleted or replaced file stays in storage until seven days after nothing names it: no question, group, material or published version, whoever owns it. The sweep then deletes the object and marks the row `purged_at`. The row stays, the file cannot be restored, and no notification follows: the teacher is told when they act, by the answers of `replaceMedia` and `deleteMedia` (`previous` and `deleteAt`, with a URL to download the file) and by `usage` (`heldBytes`, `pendingBytes`, `pendingDeleteAt`). The API runs the sweep at start-up and every hour on every machine (`jobs.SweepRetiredMedia`). It dates the files nothing names yet, cancels the wait of a file something names again, and removes the due ones one at a time under the row's lock. A row that a second sweep or a writer holds is skipped, so no object is deleted twice, and a publish that freezes a file again before the sweep takes it keeps it. A delete that fails leaves the row for the next run, ten minutes later. A purged file can be named by nothing: `LockForVersionUse` and `Readable` answer not found, and the database refuses the row. An import review that still names a file the sweep removed fails at its commit as one that is gone. `MEDIA_RETENTION_SWEEP=off` stops the sweep on the machines that read it; the quota and the dating by replace and delete go on.
- **The library says it before the action and after it (F-68, DG-206).** The Replace dialog says what becomes of the old file: kept for the N published tests that use it, each test counted once however many versions hold the file, and deleted seven days after none does; or, when none does, deleted for good seven days after the replacement. The Delete dialog says the file is deleted for good seven days later and cannot be restored; a file a published version uses is still refused. Both offer the file's download from the row's URL. After the action the toast names the date the file goes (`previous.deleteAt`, `deleteAt`), or that it is kept for the tests that hold it. The Storage card counts `heldBytes` in the bytes used and lists the held and the pending files, with `pendingDeleteAt` as the earliest day. A replacement refused for room (409) while published tests hold the file says the library needs room for both files, and a 429 says how long to wait from `Retry-After`, which the upload transport reads. Nothing is announced by the bell and nothing restores a file.
- Honest note: `controlsList="nodownload"` and hiding the URL are **not** protection. A determined student can save the file. Signed short-lived URLs raise the cost slightly; do not invest further. If a listening file must never leak, do not put it online.

### 11.3 Player component

`web/src/features/media/components/AudioPlayer.tsx`. Custom controls over a native `<audio>`; no third-party audio library.

- Controls: play/pause, elapsed/total time, a progress bar that is **display-only when `allowSeek` is false**, and a plays-remaining indicator. The total shown is the server's probed length when it is known; the file's own length drives the track and the seek.
- The engine draws it as the deck's recording card (`layout="card"`), for a question's own audio and for a shared recording alike: a 52px round button named Play, Pause, Resume (a paused play in progress) or Play again (after an end), a 6px track in the brand accent with the seek control over it when `allowSeek` is on, the time and a counter under it ("Play 1 of 2", which reads 1 before the first play; "Play n" with no limit; "Played n/N times" at and past the limit), and a status line: "Press play when you are ready.", "Playing", "Paused", "Finished · 1 play left" (or "Finished" with no limit) and, at the limit, "You have used all N plays." with "You can keep listening. Additional plays are recorded for your teacher to review." The button stays live at the limit (§11.4). A refused play's sentence takes the status line's place.
- `preload="metadata"` so duration renders without downloading the file.
- **Autoplay is impossible** — browsers block audio without a user gesture. The first play is always a tap. Do not attempt to auto-start; do not treat the block as an error state.
  A play refused with `NotAllowedError` says so under the player, which stays
  in place. `AbortError`, including a pause before playback started, says
  nothing. Other playback rejections or a file-load failure show the
  expired-link card.
- iOS Safari: only one audio element plays at a time, and playback must originate from a gesture handler (not an async continuation). Call `.play()` synchronously in the click handler; do not `await` anything before it.
- `allowPause: false` (§11.1): while the recording plays the button is inert, marked `aria-disabled` (never the native `disabled`, so it keeps its focus and its name), at 45% opacity, and a press does nothing; the status reads "Playing · this recording can’t be paused", and before the first play "Press play when you are ready. It can’t be paused once it starts." It cannot stop playback stopping: a headset or OS media button, another recording starting, leaving the question or the group, or a lock still pauses it. The card then reads "Paused", and Resume continues the play in progress without spending a play. Nothing new is recorded: the product reports what it can see and does not pretend the pause could not happen. A payload without the field, from an API older than the tab, pauses freely.
- `allowSeek: false` implementation: no `<input type="range">`, and an `onSeeking` handler that resets `currentTime` to the last known position. Note that OS-level media controls can still seek in some browsers — record an `audio_seek` event rather than pretending it cannot happen.
- Accessibility: real `<button>` elements, `aria-label` on each, keyboard-operable, `aria-live` announcement of plays remaining. When `showTranscriptAfterSubmit` is on, the transcript appears on the result page — this is also the accessibility fallback for hard-of-hearing students.
- One player instance per question, and one per shared recording, which stays mounted while the student moves between its group's questions. Leaving the question or the group, by any means, pauses and releases it. A play in progress and its position are remembered in memory, per question or recording (`media/playback.ts`), so coming back to the question or the group resumes there without spending a play. Leaving the test page, by any means, forgets every place, as a reload does: coming back to the same attempt, or starting another, spends a play. Starting one player pauses any other that is playing.
- Each play that starts the recording sends an `audio_play` event; resuming a paused play sends none.

### 11.4 `maxPlays` — server-authoritative

The obvious client-side counter resets on reload, which makes the limit meaningless. So:

- A play is spent when playback starts and no play is in progress: from 0:00, or after the recording ended. A resume after a pause spends none. With seeking allowed, moving inside a play in progress spends nothing. A remembered place (§11.3) is put back once the element has its metadata, and the no-seek guard is told the new position first, so a restore is never reported as a blocked seek. If playback then runs more than 0.5s short of the remembered point (the browser refused the seek, or the metadata never came), the start is counted as a play after all and the place forgotten: a place that cannot be put back is never free.
- Each play calls `POST /app/attempts/:id/audio-play`; the server increments `attempt_audio_plays (attempt_id, question_id, plays)` and the count is returned in `GET /app/attempts/:id` as `audioPlays`. The `audio_play` event the player records beside it (§10.6) only reports the play and changes no count.
- Client renders remaining plays from the server value, optimistically decrements on play, and reconciles on the next fetch.
- Playback is **optimistic**: a failed event POST does not block the audio. A student who goes offline to farm replays will show a gap in the event log, which is exactly what the integrity timeline is for. Blocking playback on a network round-trip would punish bad wifi far more often than it would catch anyone.
- On submit, the server rejects nothing based on play count. Over-limit plays are reported to the teacher, not enforced retroactively.

### 11.5 Word milestone shared recordings (approved, not yet enabled)

D-03 extends the existing audio contract to a recording shared by a group. All
child questions consume the same configured play allowance. Navigating between
children, reloading or resuming the same attempt on another device never grants
a fresh allowance. Two groups may reference the same immutable file and still
have independent counters. The counter identity therefore includes the versioned
group recording binding; the asset ID alone is insufficient.

Freeze that binding and its audio policy in the published version, and keep its
counter scoped to the attempt. Preserve synchronous gesture playback, optimistic
accounting, retry deduplication, teacher reporting and transcript visibility rules
from §11.3–11.4. The future shared player must use this scope consistently in
navigation, event writes and state reconciliation. Its API, relational references,
concurrency tests and student-payload tests must precede enabling shared audio.
Existing per-question audio remains a separately supported historical contract.

W-09c adds a separate shared-play endpoint with `recordingId`, current `sessionId`
and a stable per-gesture `playId`. It follows the existing autosave writable-session
checks and verifies the recording's material binding belongs to the attempt's
version before writing. Retrying the same gesture/recording returns its current
counter; reusing the gesture ID for another recording is a conflict. The counter,
append-only receipt and server `audio_play` event commit together. Counts survive
reload/takeover and are returned as `groupAudioPlays`, keyed by recording ID.
Teacher monitor and timeline excess-play totals include this ledger. New attempts
start with their own allowance. Network acknowledgements and `maxPlays` never gate
the actual browser play call. The browser persists unconfirmed gesture IDs per
learner/attempt until the attempt deadline, clears them on logout and serializes
bounded retries. Confirmed extensions update the recovery deadline. A pending
sync label is distinct from answer save state. A response lost after commit does
not consume a second allowance on retry; late responses cannot mutate another
session. Counts remain monotonic during refetch. Submission attempts a bounded
three-second telemetry flush but remains available during network accounting
failure. Closed/expired attempts reject late telemetry; offline closure cannot
guarantee complete listening evidence. Results and teacher paper review report
the confirmed counts without creating new playback receipts.

The assignment introduction derives listening presence, transcript permission and
the strictest finite allowance from both question audio and shared recordings on
the assigned version. A newer default does not change these facts. The displayed
minimum is not presented as every recording's allowance; individual players show
their own limits. A shared-audio notice explains that member navigation and reload
do not grant a new allowance.

---

## 12. Design guidelines

Deliberate. Do not "improve" them with trendy defaults.

**As built in R1 (v0.7.0).** The deck (`docs/design/deck/`) is the source. The organization logo is stored and served as a PNG (§5.6).

- **Tokens.** `web/src/index.css` carries the deck's light and dark sets. `tokens.test.ts` measures them rather than pinning values:
  - every themed token has a dark value;
  - the neutrals stay under 0.03 chroma and `--primary` stays charcoal;
  - each colour family stays in its hue band, with only info allowed to be blue;
  - text pairs reach 4.5:1, and the focus ring and input borders 3:1, in both themes;
  - `@theme` routes every utility colour through a variable.

  `no-raw-colours.test.ts` refuses a Tailwind palette class, or a literal hex, `rgb()`/`rgba()` or `oklch()` colour, in the `.ts`/`.tsx` files under `src/` (except `GoogleMark.tsx`); the literal values live only in `src/index.css`.
- **Accessibility tokens beyond the deck** (DG-30, DG-31), each measured in both themes:
  - `--focus`, the 2px `:focus-visible` ring on every control, 3:1 or more on `--bg`, `--card`, `--sidebar` and `--muted`; a control paused with `aria-disabled` stays focusable and fades its colours, never the element's opacity, which halves the ring (`paused-opacity.test.ts` holds the sites that still do to a baseline that only goes down);
  - `--input`, input borders, 3:1 or more;
  - `--danger-solid` / `--danger-solid-fg`, danger buttons, 4.5:1 or more.
- **Deck geometry that would change an existing primitive** (control heights, `--radius` and the sm/md/lg radii, the badge and card shapes) applies only inside `data-scale="deck"`, which rebuilt surfaces set on their root. The deck's type steps are global tokens, but the pre-R1 steps keep their values, so an old screen does not shift. Sizes that exist only in the deck have their geometry everywhere (button `md`, `xl` and `icon-xl`, the new avatar sizes, the toast).
- **Every console follows the theme.** The teacher's `TeacherLayout` is a deck surface since R4, and the student and focus layouts since R3; nothing forces light. R5 makes the deck geometry the default and removes `data-scale` (T-R5.30).

**The teacher workspace's page frame** (DG-205, Decided: the design team's page-width design, `docs/design/deck/page-width-proposal/`, approved by the owner on 2026-10-12; the v2 deck drew 1320px on most screens and 1080, 860 and 720 on some, each centred).

- **One frame, up to 1600px.** `TeacherLayout` puts every route in the same frame: the full content width up to `--page-max` (1600px), then centred, at every width, 2560 included. So every page, its loading, empty, error and not-found states included, starts at one left edge and has one width at each viewport and sidebar state. The one named exception is the import review, full-bleed with the sidebar collapsed (`handle.width: "full"`). The values are tokens in `src/index.css`; no other route or page sets a maximum, and `page-frame.test.ts` refuses a teacher page that centres itself in a narrower box.
- **Gutters by width:** `--page-gutter` 14px below 768, 28px from 768 and 40px from 1920; top padding 16 / 24px.
- **Size the content, not the page.** The page head (title, description, actions, the wizard's stepper) spans the frame, and cards, lists, tables and steppers fill it. Only text is capped: a field is at most `--field-max` (480px) wide, prose at most `--prose-max` (72ch). There are no form columns: New assignment's summary docks on the right at 300px beside a form that fills, and Settings' fields flow into at most three columns, each field at most 480px. Assignments shows Assigned to from 1240px of content, Submitted from 760 and To grade from 900; the Imports history adds Source from 1240. Inner reflows follow the content-width bands of §8 or a container query, never a new viewport breakpoint.

**The AI assistant's panels** (D4; DG-201). Built in T-D4.34 (the panel), T-D4.35 (the question editor's suggestions), T-D4.36 (the grading card) and T-D4.37 (Explain results). The server enforces what it can (§15); these are the rules the web draws.

- **The acceptance rule.** What the assistant returns is a suggestion, never a record. A panel's primary button saves only the items the teacher kept, through the writes that already exist (the section and bank question writes, `updateQuestion`, `duplicateTest`, the bulk tag write), and `features/ai/` itself calls nothing but `/teacher/ai/*`. "Use this" and "Use score and comment" fill a field and save nothing; the teacher's own Save does. On the grading card the filled score and comment are the answer's draft, unlike a score button, which saves at once: Save & next writes them, and so does moving to another answer, as it writes a typed comment; leaving the page with a filled score unsaved asks first, as it does for a typed comment. A question the assistant drafted carries the tag "AI draft" while the preference for tagging is on, until a teacher edits or approves it. No sample output is ever drawn.
- **The disabled-with-reason rule.** An entry point is hidden only when the server has no assistant (`PublicStatus.ai` is false) or the caller lacks `ai.use`. For a teacher who is paused or out of credits it stays visible and disabled, with the reason as its tooltip and in the panel's footer: "AI is paused for your account.", "Your allowance is used up. Ask for more in Settings." or "The school's AI pool is used up this month." (`AI_PAUSED`, `AI_CREDITS_EXHAUSTED` with `details.scope`). The two features that send a student's answer text are not offered while the teacher's "Let AI read student answers" is off: in place of "Explain results with AI" the Questions card says in one line that it needs the switch, with a link to Settings › AI assistant, and the grading card says in one line that suggestions are off in Settings › AI assistant.
- **The question editor's suggestions** (V-10), in the bank's editor and the builder's pane, which share the editor (DG-108). "Write with AI" sits in the Explanation's label row and "Suggest wrong options" beside Add option on a single or multiple choice question; each opens an inline panel under its field, not a drawer. Before asking, the editor checks the deck's two guards as toasts ("Write the question first", "Mark the correct answer first"). Both operations read the stored question, so the builder saves its pending edit first, and the bank, whose Save is explicit, asks for a save ("Save the question first") while what the assistant reads (the type, prompt, written options, accepted answers and sample answer) differs from what is stored; an empty option is not read and does not count. The explanation is written in the teacher's preferred language; "Shorter" keeps the language of the one shown and "In Vietnamese" / "In English" asks for the other. "Use this" replaces the field's text in the form it holds (rich content, or Markdown) and "Add" fills the first empty option that is not marked correct, or adds one while there are fewer than eight (DG-63); neither saves, so the builder's autosave or the bank's Save writes them. A panel that answered ends with the call's credit note. A failure, or an answer with no suggestion, is a line in the panel; the button is the way to try again.
- **The credit note.** A panel's footer says what the call cost and what is left, from the answer's `usage` and in credits, never in money. The deck's costs are placeholders: the product shows no figure it has not metered.
- **Failure.** A provider that did not answer, refused, or answered something that fails the schema is the warning callout "AI didn't respond. Try again in a minute." with "Try again" (`AI_UNAVAILABLE`, which asks for 30 seconds). A panel closes on Esc, keeps focus inside while open and returns it to the button that opened it (DG-36).

**The student app and the engine, as built in R3 (v0.9.0) and D4.** The source is `Quizzivy Student.dc.html`. What the deck does not draw, and where the product departs from it, is in `docs/design/gaps.md` (DG-80 to DG-89, DG-102 to DG-107, DG-119, DG-182 to DG-194).

- **One breakpoint for the shell, 768px.** The student shell changes its chrome there and nowhere else (destinations in the top bar or bottom tabs), and so does the engine's: side-by-side panes or the drawer over the question, and the sheet as a card or a bottom sheet. A page keeps its state across it.
- **The engine's other thresholds** are its own, read through one width source (`useEngineWidthAtLeast`): the window's width, or a preview frame's (§7.3). The sections rail shows from 900px on a question with no shared content and from 1180px on one with it, and the grid button with its sheet stands in for it everywhere else; the grid button's label takes its short form below 420px (and beside shared content from 768); the drawer is 88% of the width, at most 420px; the engine's 1024px sizes (`engine-lg:`, below) follow the same source. Crossing any of them remounts nothing and keeps what is typed.
- **One centred column per page:** Home and Classes at most 960px, the test intro 720px, the result 820px, Settings 760px. No page has a side panel. In the engine the passage column is at most 640px and the question column 600px beside shared content, 720px alone.
- **Classes** are cards in a grid that fills the row, with 280px as a card's least width. A card is not a link.
- **Touch targets.** Below 1024px the student surfaces put a 44px floor on buttons, and so does the engine in a teacher's preview frame narrower than 1024px, whatever the window's width. A control the deck draws keeps the deck's size: the header's 36px ✕, the 32px flag toggle, the rail's 34px squares (never below 900px), the drawer's 30 × 84px handle and 34px close button, the dialogs' 42px buttons (46px for the one button of the "you left the test" alert). The engine's Previous and Next and the grid button are 44px, and the question sheet's squares 44px below 1024px and 42px from it. An option row is at least 52px high; a cloze card's option row is 48px.
- **Dialogs** use the deck's frame: 440px (420px for the "you left the test" alert) or the width less 24px, no close button, the actions at the right; the alert has one button, as wide as the dialog. A dialog with a field sits 12% from the top below 768, so the keyboard does not cover it (DG-103).
- **The timer** is a pill centred in the header's free space. Each digit sits in a cell one zero wide, because Be Vietnam Pro has no tabular figures and the pill would otherwise change width every second. Under five minutes it takes the danger tones.
- **Text in a test.** The passage is 16px on a 1.75 line and cannot be selected, the prompt 17px, an option 15px. "Chữ lớn hơn khi làm bài" raises them to 18, 19 and 17px; the signed-in choice is an account preference mirrored in this browser; anonymous choices remain local.
- **Content keeps a light paper surface in dark mode** (DG-35): images and rich tables in the engine and on the result.

The rules below carry over from v0.43, restated in the deck's tokens, except that the shadow and radius limits now follow the deck. Typography, motion, the front door, dark mode and the lime accent are new in R1.

- **Neutral by default.** The deck's teal-grey scale (`--bg`, `--card`, `--muted`, `--border`, `--fg`, `--muted-fg`) and 1px borders. Deck surfaces use the deck's shadows (`shadow-card` at rest); floating surfaces go up to `shadow-lg`. The old consoles keep shadcn's shadows until their rebuild.
- **Primary action: the deck's charcoal `--primary` with `--primary-fg`.** Not blue, not purple, not indigo. The lime `--accent-c` (with its soft and ink tones) marks progress, counts and current states, never a primary button.
- **Semantic color only where it carries meaning:** green = correct/success, red = incorrect/error/destructive, amber = warning/time-low. Never decorative.
- **Forbidden:** decorative gradients, glassmorphism/backdrop blur, pulsing rings, glow effects, radii beyond the deck's (the old consoles keep `rounded-md` controls and `rounded-lg` cards), emoji in UI chrome.
- **Typography:** Be Vietnam Pro, self-hosted (latin, latin-ext and vietnamese subsets, 400–700, upright and italic; the CSP allows no font host). Every character the UI draws is in the loaded font faces; a symbol the font lacks is an icon (`KeyGlyph` for ⌘, ↵, ↑ and ↓), or a character it has, such as the en dash for a range. Content is shown in NFC: the server composes the text a person types before it stores it, the rich content editor composes what it writes, and a view composes what it draws, because a row saved before the server did can still be decomposed. The deck's scale: 3xs 10.5, 2xs 11, caption 11.5, xs 12, meta 12.5, sm 13, ui 13.5, base 14, body 14.5, md 15, title 16, lg 17, stat-sm 18, xl 20, stat 22, h1 24, h1-student 26, kpi-sm 28, kpi 30, display 34px. Phone text inputs stay at 16px to avoid input zoom; student touch targets follow the R3 rule above; seek tracks have a 44px hit area. `leading-relaxed` in the test view.
- **Icons:** lucide-react, 16px dense / 20px nav, consistent stroke, `aria-hidden` unless standalone.
- **Density:** admin tables dense (~40px rows). Student pages are the centred columns of the R3 rules above. The teacher's pages follow the deck's tables and cards inside the content-width rules of §8. The student test view stays spacious: one question at a time (a cloze's run is one screen), beside its shared content from 768px when it has some.
- **Action hierarchy:** resume is primary; opening an assignment's intro is secondary. Deadline pills turn amber only within 24 hours. The 320px test footer has previous as an icon, the grid button that opens the question sheet, and Next or Finish. In the Submit dialog the two buttons stay in reach while a long list of unanswered questions scrolls. Submission confirmation offers the submitted paper directly.
- **Explicit states:** single- and multiple-choice instructions identify selection behaviour without revealing the key. The save line says saved, saving, offline or failed; exhausted audio allowances explain continued playback is recorded. Login exposes class joining and a reversible password visibility toggle. Signed-in join screens identify the current account and provide a way back.
- **Motion.** The deck's keyframes, and what the old consoles still carry:
  - 150ms control feedback, and the settings panels' 150ms ease-out `settings-enter` entrance.
  - A 2px hover lift on cards, on pointer devices (ours; the deck changes the border instead).
  - The splash's .35s fade and its bar.
  - `qz-breath`, `qz-indet` and `qz-shimmer`: the splash, and skeletons on deck surfaces. Elsewhere the skeleton keeps shadcn's `animate-pulse`; spinners use Tailwind's `animate-spin` (`qz-spin` is defined and not yet used).
  - `qz-pulse`, for the live dot.
  - `qz-marquee`, only when a title overflows, paused on hover and focus.
  - The vendored primitives' enter and exit (dialog, popover, select, dropdown menu), standing in for the deck's popover entrance.
  - The test builder outline's drag-and-drop reorder slide (dnd-kit); the deck draws a drop line instead.

  Under `prefers-reduced-motion` the marquee, live dot, breath, shimmer and indeterminate bar each have a static state, not a shorter one: an ellipsis, a solid dot, a still mark, flat `--muted`, a fixed 40% bar. Everything else is cut to 0.01ms by the global reduced-motion block. Exam inputs remain stationary. The rebuilt consoles (R3–R5) replace the old consoles' motion with the deck's.
- **Audio player:** monochrome. A filled `--primary` play button, a thin `--secondary` track with a `--primary` fill. The engine's recording card (§11.3) is the one exception, as the deck draws it: its 6px track on `--muted` fills with the lime `--accent-c`, which marks progress. No waveform visualisation, no equaliser animation, no other colored accents.
- **Front door:** sign-in, forgot password, join and change password share the brand frame, with the brand panel from 900px, except join, which stays one column. On join, the class name is large and there is one primary button. This is the first thing a new student sees — it should look calm and legitimate, not like a marketing page.
- **Dark mode:** in scope from R1.
  - The preference is light, dark or system, stored in `localStorage['quizzivy.theme']`.
  - `web/public/boot.js` applies it before paint, since the CSP allows no inline script.
  - Components read colour tokens rather than hard-coding colours. The exceptions: the brand art, which shows its on-dark files in dark because an image cannot read a token; `GoogleMark`, in Google's own colours; and the vendored primitives in `components/ui/`, which keep their `dark:` variants.
  - Content surfaces keep a light `--paper` surface in dark mode (DG-35). The tokens exist, and each surface adopts them when it is rebuilt.
- **Empty states:** one short sentence + one primary action. No illustrations.
- **Vietnamese first.** Design for longer Vietnamese strings; avoid fixed-width labels.
- **Integrity UI follows the deck** (D8, decided 2026-09-26). On the student side since R3: the red timer under five minutes, the "Bạn vừa rời trang làm bài" dialog with its eye-off tile, "Giáo viên đã được báo" past the allowance under `flag`, and the danger toast for a blocked copy or paste (§10.2). The words stay plain: no sentence a student reads says "violation" or "cheating", and the product never concludes that a student cheated. The teacher's roster follows in R4; until then it keeps the calm rule: plain text, no alarm iconography.

---

## 13. Database design (PostgreSQL 18)

### 13.1 Binding references

Two authorities govern all schema and query work. Where they conflict, the PostgreSQL docs win.

1. **Neon `postgres-best-practices` agent skill** — <https://github.com/neondatabase/postgres-skills>. Install before writing any DDL:
   ```
   npx skills add neondatabase/postgres-skills
   ```
   An Agent Skill package (`skills/postgres-best-practices/SKILL.md` + `references/schema-design.md`), curated by Postgres practitioners including a PostgreSQL Core Team member. Its schema-design, indexing, and migration guidance is binding for this project.
2. **PostgreSQL 18 official documentation** — <https://www.postgresql.org/docs/18/index.html>. Version-specific behaviour is checked here, not against training-data recollection. Target **PostgreSQL 18**; do not write DDL that silently degrades on 16/17.

**Rule for the agent:** load the skill before proposing DDL or non-trivial SQL, and cite the specific docs section for any PG18-specific construct used.

### 13.2 Conventions

- Schema `app`, not `public`. `REVOKE CREATE ON SCHEMA public FROM PUBLIC`.
- Tables plural snake_case; columns snake_case; no Hungarian prefixes.
- PKs: `uuid PRIMARY KEY DEFAULT uuidv7()` — PG18's built-in time-ordered UUID avoids the B-tree fragmentation random v4 causes on insert-heavy tables. Use `uuidv4()` only where hiding creation time genuinely matters.
- `timestamptz` everywhere; never bare `timestamp`. Store UTC, render `Asia/Ho_Chi_Minh`.
- `text` over `varchar(n)` unless a real business constraint exists; enforce with `CHECK` when it does.
- Scores: `numeric(8,2)`. Never `float`.
- PG enums where the set is genuinely closed (`attempt_status`); lookup tables otherwise, such as `roles` (R2), which custom roles keep open. Adding an enum value is easy, removing one is not.
- Every FK has an explicit `ON DELETE`. Default `RESTRICT`; `CASCADE` only for true owned children.
- Every FK column used in a join or filter gets an index — Postgres does not index FKs automatically.
- `created_at timestamptz NOT NULL DEFAULT now()`; `updated_at` via trigger, not application code.
- Soft delete only where history matters (`questions`, `tests`, `media_assets`); everything else deletes for real.

### 13.3 Core tables

Sketch, not final DDL. The agent produces real migrations after loading the skill.

```sql
CREATE SCHEMA app;

CREATE TYPE app.test_status    AS ENUM ('draft','published','archived');
CREATE TYPE app.question_type  AS ENUM ('single_choice','multiple_choice','true_false','fill_blank','short_answer');
CREATE TYPE app.attempt_status AS ENUM ('in_progress','submitted','timed_out','graded','voided');
CREATE TYPE app.media_kind     AS ENUM ('image','audio');
CREATE TYPE app.join_source    AS ENUM ('admin','join_code');

CREATE TABLE app.users (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  email         text NOT NULL,
  full_name     text NOT NULL,
  role_id       uuid NOT NULL REFERENCES app.roles(id) ON DELETE RESTRICT,
  password_hash text,                              -- NULL = Google-only account
  must_change_password boolean NOT NULL DEFAULT false,
  disabled_at   timestamptz,
  session_epoch integer NOT NULL DEFAULT 0 CHECK (session_epoch >= 0),
  created_by    uuid REFERENCES app.users(id) ON DELETE SET NULL,  -- NULL = unknown or self-joined
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON app.users (lower(email));

ALTER TABLE app.users
  ADD COLUMN display_name text CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 80),
  ADD COLUMN phone text CHECK (phone ~ '^[0-9+ ]{6,20}$'),
  ADD COLUMN avatar_key text,
  ADD COLUMN locale text CHECK (locale IN ('vi', 'en')),
  ADD COLUMN time_zone text CHECK (char_length(time_zone) BETWEEN 1 AND 64),
  ADD COLUMN preferences jsonb NOT NULL DEFAULT '{}'::jsonb
    CHECK (jsonb_typeof(preferences) = 'object')
    CHECK (octet_length(preferences::text) <= 8192);

CREATE TABLE app.user_identities (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  provider         text NOT NULL CHECK (provider IN ('google')),
  provider_user_id text NOT NULL,                  -- Google `sub`, immutable
  email_at_link    text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id)
);
CREATE INDEX ON app.user_identities (user_id);
```

**Set-password links** (§5.4), from R5 (`00135_create_set_password_tokens.sql`, `docs/plan/20-data-model.md` D-55, §53):

```sql
CREATE TABLE app.set_password_tokens (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL,                       -- SHA-256 of a 256-bit token
  purpose    text NOT NULL CHECK (purpose IN ('invite', 'reset')),
  created_by uuid REFERENCES app.users(id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT set_password_tokens_token_hash_key UNIQUE (token_hash),
  CHECK (length(token_hash) = 32),
  CHECK (expires_at <= created_at + interval '24 hours')
);
CREATE INDEX set_password_tokens_user_idx ON app.set_password_tokens (user_id);
```

A link is used by one `UPDATE … SET used_at` that names the hash and finds it unused, unrevoked and unexpired, so two submits of one link set the password once. `used_at`, `revoked_at` and expiry read alike to the lookup. The app role may read, insert and update a link, and the table stores no secret a reader could present.

**Roles and permissions** (§5, plan 70 §4), from R2:

```sql
CREATE TABLE app.permissions (
  key       text PRIMARY KEY,                      -- 'content.tests.write', ...
  group_key text NOT NULL CHECK (group_key IN ('content','teaching','people','system')),
  ordinal   smallint NOT NULL CHECK (ordinal > 0),
  in_matrix boolean NOT NULL,                      -- false for the four hidden keys
  UNIQUE (group_key, ordinal)
);

CREATE TABLE app.roles (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  builtin_key text UNIQUE CHECK (builtin_key IN ('admin','teacher','assistant','student')),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 60),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 300),
  icon        text NOT NULL,                       -- one of the Edit role dialog's twelve
  color       text NOT NULL,                       -- one of its seven card colours
  copied_from uuid REFERENCES app.roles(id) ON DELETE SET NULL,
  revision    bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_by  uuid REFERENCES app.users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX roles_name_lower_key ON app.roles (lower(name));

CREATE TABLE app.role_permissions (
  role_id        uuid NOT NULL REFERENCES app.roles(id) ON DELETE CASCADE,
  permission_key text NOT NULL REFERENCES app.permissions(key) ON DELETE RESTRICT,
  PRIMARY KEY (role_id, permission_key)
);
```

`permissions` is plan 70 §4.1's catalogue. A migration writes the catalogue, the four built-in roles and their grants (`docs/plan/20-data-model.md` D-21). A custom role has no `builtin_key`, and no role's `builtin_key` changes. The Admin stores no grant but `learning.take_tests`: it holds every other key as the wildcard. Triggers refuse a hidden key for any role and keep `learning.take_tests` on the built-in Student, and every grant change bumps the role's `revision`. The view `app.student_like_roles` is the one definition of a strict student target (§5.4): the built-in Student, or a custom role holding nothing but `learning.take_tests`.

`users.role_id` replaced `users.role`. Migration 00105 (T-R3.1) validated its `NOT NULL` and dropped `role`, its index, `app.user_role` and the trigger that kept the two in step for the v0.7.0 binary. The trigger `users_last_admin` refuses a demotion, disable or delete that would leave no active Admin. An access token carries the user's `session_epoch`, and a token below the stored value is refused; a disable and a staff password reset bump it (§5). `created_by` names the staff member who created the account; it is NULL for an account older than R2 and for a Google self-join. Details: `docs/plan/20-data-model.md` §29–§30.

**Classes and join codes** (§6):

```sql
CREATE TABLE app.classes (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL,
  description       text,
  schedule_label    text CHECK (char_length(schedule_label) BETWEEN 1 AND 120),
  room              text CHECK (char_length(room) BETWEEN 1 AND 60),
  self_join_enabled boolean NOT NULL DEFAULT true,
  teacher_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.class_join_codes (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  class_id        uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  code_hash       bytea NOT NULL,                  -- 1: sha256(normalized code); 2: HMAC-SHA256
  code_ciphertext bytea CHECK (code_ciphertext IS NULL OR length(code_ciphertext) = 36),
  key_id          smallint CHECK (key_id IS NULL OR key_id <> 0),
  lookup_scheme   smallint NOT NULL DEFAULT 1 CHECK (lookup_scheme IN (1, 2)),
  code_hint       text NOT NULL,                   -- last 4 chars, for the teacher's display
  expires_at      timestamptz NOT NULL,
  max_uses        integer CHECK (max_uses IS NULL OR max_uses > 0),
  uses_count      integer NOT NULL DEFAULT 0,
  revoked_at      timestamptz,
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (code_hash),
  CHECK ((lookup_scheme = 2) = (code_ciphertext IS NOT NULL)
     AND (lookup_scheme = 2) = (key_id IS NOT NULL))
);
-- one active code per class
CREATE UNIQUE INDEX class_join_codes_one_active
  ON app.class_join_codes (class_id)
  WHERE revoked_at IS NULL;

CREATE TABLE app.class_members (
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  joined_via app.join_source NOT NULL,
  joined_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, user_id)
);
CREATE INDEX ON app.class_members (user_id);
```

The code is stored encrypted, so it can be read back for its teacher and for admins (D5). A code issued from v0.8.0 on is a scheme-2 row: `code_ciphertext` holds a 12-byte nonce, the 8-byte code and a 16-byte AES-256-GCM tag, sealed under `JOIN_CODE_KEY` with the class and code ids as additional data; `key_id` is the 16-bit id derived from that key; and `code_hash` is the HMAC-SHA256 of the normalized code under a lookup key derived from it. The key never reaches the database, so a dump does not hand over class access. A code issued before v0.8.0 is a scheme-1 row: its SHA-256 and no ciphertext. It redeemed and could not be read back; v0.10.0 replaced every live one at start-up with a scheme-2 code (§6.1, D5), and a revoked or expired scheme-1 row stays until R5 removes the lookup. `code_hash` stays the only lookup path (§6.5). Key derivation and rotation: §6.1 and `docs/plan/20-data-model.md` §33.

**Media** (§11):

```sql
CREATE TABLE app.media_assets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  kind              app.media_kind NOT NULL,
  storage_key       text NOT NULL UNIQUE,          -- R2 object key; immutable
  mime_type         text NOT NULL,
  bytes             bigint NOT NULL,
  duration_ms       integer CHECK (duration_ms IS NULL OR duration_ms > 0),
  original_filename text NOT NULL,
  display_name      text CHECK (char_length(display_name) BETWEEN 1 AND 200 AND display_name ~ '\S'),  -- NULL: the library shows original_filename; D-29
  default_max_plays smallint CHECK (default_max_plays IS NULL
                      OR (default_max_plays BETWEEN 0 AND 3 AND kind = 'audio')),  -- 0 is unlimited; D-29
  replaced_by       uuid REFERENCES app.media_assets(id) ON DELETE SET NULL CHECK (replaced_by <> id),  -- the successor of a replaced file; D-29
  width             integer,                       -- an image's pixels, both or neither; D-29
  height            integer,
  checksum_sha256   bytea NOT NULL,                -- dedupe identical re-uploads
  uploaded_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  owner_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  unreferenced_at   timestamptz,                   -- when a deleted or replaced file was last seen named by nothing; its object goes seven days on; F-68
  purged_at         timestamptz,                   -- when the sweep removed its object; the row stays; F-68
  CONSTRAINT media_assets_purged_check CHECK (purged_at IS NULL OR unreferenced_at IS NOT NULL),
  CHECK (kind <> 'audio' OR duration_ms IS NOT NULL),
  CONSTRAINT media_assets_bytes_by_kind
    CHECK (bytes > 0 AND bytes <= CASE WHEN kind = 'audio' THEN 52428800 ELSE 10485760 END),
  CONSTRAINT media_assets_dimensions_paired CHECK ((width IS NULL) = (height IS NULL)),
  CONSTRAINT media_assets_dimensions_positive CHECK (width > 0 AND height > 0),
  CONSTRAINT media_assets_dimensions_image_only CHECK (width IS NULL OR kind = 'image')
);
CREATE INDEX ON app.media_assets (kind, created_at DESC);
```

The library's fields are R4's (migrations 00082 to 00084, D-29). `display_name` is what the library shows and renaming changes it alone; `original_filename` and the stored object never change. `default_max_plays` is an audio file's default play limit, 0 to 3 with 0 unlimited, which the editor starts a question's own limit from and which changes no question. `replaced_by` is set on the old row when a file is replaced: that row stays, because published versions still reference it, and it leaves the library list and the owner's quota (§11.1). `width` and `height` are read from an image's header at upload, so a row stored before 00083 has none. The size limit depends on the kind, 50 MiB for audio and 10 MiB for an image. One owner's library may hold `MEDIA_OWNER_QUOTA_MIB` (5 GiB by default), counting the files in it and the deleted or replaced files that something still names; the writers of one library take advisory lock `(73820, hashtext(owner))` before they sum it (`docs/plan/20-data-model.md` §28). `unreferenced_at` and `purged_at` are the retention of a file that left the library (§11.2; migrations 00111 to 00113, `docs/plan/20-data-model.md` §45): a row that names a purged file is refused by a trigger on the six tables that name a file, and one that names a file waiting for its removal clears the date.

**Question bank** — normalize; do not stuff options into `jsonb`. Options and blanks are ordered, queried, and graded against.

Migration `00032_add_option_content.sql` adds nullable `content jsonb` to
`question_options` and `test_version_options` for the bounded inline AST only.
Option identity, order, text and correctness stay relational. No grading key is
placed in the AST, and no legacy row is rewritten. Read-compatible binaries are
the rollback floor after rich writes; rolling back the migration discards rich
formatting and is only a pre-rollout development operation.

Migration `00033_add_question_content.sql` adds nullable `prompt_content jsonb`
and `explanation_content jsonb` to `questions` and `test_version_questions`.
These columns hold the bounded prose AST from §7.1; companion strings remain
exact plain projections. Migration `00034_add_question_gap_bindings.sql` enables
rich fill-blank prompts and adds nullable, question-scoped unique `gap_id` to
`question_blanks` and `test_version_blanks`. Null identifies legacy Markdown.
Bank copies and restored drafts remap gap identities and answer bindings together;
snapshots freeze them. Answer submissions remain keyed by frozen blank UUID.
Down refuses existing rich fill-blank rows instead of silently discarding bindings.
No legacy rows are rewritten, no grading keys move, and the same read-compatible rollback floor
applies after prose writes.

Migrations `00114` to `00117` (D4, T-D4.13) add the pause rule of §11.1, one column each. `questions.audio_allow_pause` and `test_version_questions.audio_allow_pause` are nullable `boolean`s with no default, no backfill and `CHECK (audio_allow_pause IS NULL OR media_asset_kind IS NOT DISTINCT FROM 'audio')`, and every reader takes `COALESCE(…, true)`. They are nullable because the previous binary inserts these rows during a rolling deploy and writes no value, and because `questions_audio_policy_iff_audio` is a biconditional over the other two policy columns, which this column therefore stays out of. `group_recordings.allow_pause` and `test_version_group_recordings.allow_pause` are `boolean NOT NULL DEFAULT true`: a recording always has its whole policy, and a constant default is stored without a rewrite. None of the four rewrites or indexes a populated table.

```sql
CREATE TABLE app.questions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  type            app.question_type NOT NULL,
  prompt          text NOT NULL,
  media_asset_id  uuid REFERENCES app.media_assets(id) ON DELETE RESTRICT,
  media_alt       text CHECK (char_length(media_alt) BETWEEN 1 AND 1000),  -- image only; D-33
  audio_max_plays integer CHECK (audio_max_plays IS NULL OR audio_max_plays > 0),
  audio_allow_seek boolean NOT NULL DEFAULT false,
  audio_show_transcript_after boolean NOT NULL DEFAULT true,
  audio_allow_pause boolean,                       -- D4: NULL reads as true; audio only (00114)
  transcript      text,
  points          numeric(8,2) NOT NULL CHECK (points > 0),
  explanation     text,
  sample_answer   text,                            -- OQ-4; admin-only, never in a student payload
  tags            text[] NOT NULL DEFAULT '{}',
  level           text CHECK (level IN ('pre_a1','a1','a2','b1','b2','c1','c2')),  -- D-31
  skill           text CHECK (skill IN ('grammar','vocabulary','reading','listening','writing','speaking')),  -- D-31
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  owner_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX ON app.questions USING gin (tags);
CREATE INDEX ON app.questions USING gin (to_tsvector('simple', prompt));
CREATE INDEX ON app.questions (media_asset_id) WHERE media_asset_id IS NOT NULL;

CREATE TABLE app.question_options (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL,
  text        text NOT NULL,
  is_correct  boolean NOT NULL DEFAULT false,
  UNIQUE (question_id, ordinal)
);
```

`question_blanks` and `question_blank_answers` mirror this for `fill_blank`.

**Tests and versioning** — the load-bearing decision. On publish, snapshot resolved content into version tables so editing a test can never mutate an in-flight or historical attempt:

```
tests(id, title, description, status, current_version, last_published_version, owner_id, ...)
test_versions(id, test_id, version, published_at, total_points, change_note, UNIQUE(test_id, version))
test_version_sections(id, test_version_id, ordinal, title, instructions)
test_version_questions(id, test_version_section_id, ordinal, source_question_id,
                       type, prompt, media_asset_id, media_alt, audio_max_plays, audio_allow_seek,
                       audio_show_transcript_after, audio_allow_pause, transcript, points, explanation,
                       sample_answer, level, skill)
test_version_options(id, test_version_question_id, ordinal, text, is_correct)
test_version_blanks / test_version_blank_answers
```

`test_versions.change_note` is the teacher's note at publish, 1 to 200 characters, NULL when none (the command stores NULL for a blank one; D-32). `level` and `skill` (R4, D-31) are a question's optional CEFR level and skill, closed sets with no backfill; publish freezes both with the question, so a later bank edit never changes a version. Authoring accepts at most eight options for a choice question, but a stored question, a copy, a publish and a restore keep a longer array exactly as it was.

Snapshotting into **normalized rows** rather than one `jsonb` blob keeps per-question analytics a plain SQL query later. `source_question_id` preserves the bank link without coupling to it. `media_asset_id` points at the same immutable asset — the file is never copied.

**Ownership** (D3), from R2. `tests`, `questions`, `question_groups` and `media_assets` carry `owner_id`, and `classes` carries `teacher_id`. Each is `NOT NULL REFERENCES app.users ON DELETE RESTRICT` and names the teacher the row belongs to. `created_by` (`uploaded_by` for media) stays the provenance, so a transfer moves the owner without rewriting who made the row (`docs/plan/20-data-model.md` D-22). `classes` has no `created_by`; the backfill gave every existing class the oldest active Admin, the teacher v0.7.0 showed (D-23). Assignments and Word imports belong to their `created_by`. Migration 00106 (T-R3.2) validated the `NOT NULL` of every owner column, which R2 had added `NOT VALID`, and dropped the triggers that filled them for the v0.7.0 binary's inserts, so an insert that omits an owner fails. Repositories scope every read and write to what the caller reaches, and another teacher's row answers as a missing one does (plan 70 §4.2). Details: `docs/plan/20-data-model.md` §31–§32.

**Per-student overrides** (R4). `assignment_student_overrides(assignment_id, student_id, closes_at, duration_minutes, extra_attempts, reason, created_by, created_at, updated_at)`, primary key `(assignment_id, student_id)`, both foreign keys `ON DELETE CASCADE`, `created_by` `ON DELETE SET NULL`. `closes_at` and `duration_minutes` are nullable and `extra_attempts` defaults to 0; a check requires at least one of the three to change something. A student's close is `greatest(least(closed_at, closes_at), override.closes_at)`: an override only ever lengthens, and one that reaches past an early close lifts it for that student alone. `shared/schedule` holds that rule as SQL and in Go, and the join that reads it.

**Assignment options** (R4, D-34, D-35). `assignments` gains `review_release` (`on_submit` or `after_close`, default `on_submit`), `review_show_class_average` (default false) and a nullable `student_note`, 1 to 500 characters once trimmed of the whitespace JavaScript's `trim()` removes (the command stores NULL for a note that is blank by that rule, so the check never meets an empty string). The defaults are constants, so nothing is rewritten and the previous release's insert keeps its meaning. An assignment still has no `status` column: its status is a function of its window (D-18), and `shared/schedule` holds that rule once for every reader.

**An attempt in progress follows its window** (R4). `extendAssignment`, `updateAssignment` (a later close), `reopenAssignment` and `setStudentOverrides` set the deadline of each attempt in progress to the earlier of its start plus its student's time limit and its student's close, in the transaction that changed the window, and only where that is later than the deadline the attempt has (`schedule.RecomputeDeadlines`). A deadline never moves earlier: removing an override, a shorter time limit or close, and closing early leave a running attempt where it is, and so does an attempt whose deadline has passed on the database's clock, which is over whether or not it has been swept and is not revived. A deadline a maintenance window pushed past the rule stays. Each attempt moved is audited as `attempt.extended` with the old and new deadline and the cause, and an override moves only the attempts of the students it names. The writers take the assignment row and then its attempts, each attempt locked as it is read, so a concurrent submit, expiry or resume of the same attempt finishes before the deadline moves or sees the new one. An attempt starts under the window as it stands when it commits: a start racing an extension stores the extended deadline, or is itself lengthened by the extension, whichever commits first.

**Notifications** (R4, D-28). `notifications(id, user_id, kind, params, target, dedupe_key, created_at, updated_at, read_at)` holds what the app tells one user; `user_id` cascades with the user. `kind` is a dotted key of at most 64 characters, checked by pattern and not an enum, so a release adds a kind without a migration; `params` is a JSON object of at most 4096 bytes and `target` NULL or an object of at most 1024 bytes. No sentence is stored: the reader's console words the item from `kind` and `params`, and a name in `params` is a copy taken when the row was written. `UNIQUE (user_id, dedupe_key)` is how a second notice about the same thing merges into the row the user already holds. A row is kept 180 days from when it was first written; a daily job deletes older ones. `notification_preferences(user_id, event, in_app, email, updated_at)`, primary key `(user_id, event)`, holds the switches a user has saved for the five governed events; a missing row is the default, in the app and not by email. `email` is stored so that the release that sends email adds no column; nothing sends it yet.

**Word imports** (R4, D-40, D-41; the tables are `docs/plan/20-data-model.md` §23 to §26). `word_import_drafts` gains nullable `open_action_count` and `open_confirm_count` (integers, not negative), written with the review body in the same statement, which the import history reads without parsing the body; NULL is a draft not yet assessed, not zero. `word_import_sources.format` admits `text`, and `characters` (1 to 100000) is non-null exactly for a text source (§16).

**Assignments and attempts:**

```sql
CREATE TABLE app.attempts (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  assignment_id   uuid NOT NULL REFERENCES app.assignments(id) ON DELETE RESTRICT,
  test_version_id uuid NOT NULL REFERENCES app.test_versions(id) ON DELETE RESTRICT,
  student_id      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  attempt_no      smallint NOT NULL CHECK (attempt_no > 0),
  status          app.attempt_status NOT NULL DEFAULT 'in_progress',
  session_id      uuid NOT NULL,                   -- takeover detection
  started_at      timestamptz NOT NULL DEFAULT now(),
  deadline_at     timestamptz NOT NULL,            -- authoritative, server-computed
  submitted_at    timestamptz,
  graded_at       timestamptz,
  score_earned    numeric(8,2),
  score_total     numeric(8,2),
  focus_loss_count integer NOT NULL DEFAULT 0,
  flagged         boolean NOT NULL DEFAULT false,
  UNIQUE (assignment_id, student_id, attempt_no),
  CHECK (deadline_at > started_at)
);
CREATE UNIQUE INDEX attempts_one_live
  ON app.attempts (assignment_id, student_id) WHERE status = 'in_progress';
CREATE INDEX ON app.attempts (assignment_id, status);
CREATE INDEX ON app.attempts (student_id, started_at DESC);

CREATE TABLE app.attempt_answers (
  attempt_id   uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id  uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  payload      jsonb NOT NULL,
  auto_score   numeric(8,2),
  manual_score numeric(8,2),
  final_score  numeric(8,2) GENERATED ALWAYS AS (coalesce(manual_score, auto_score)) VIRTUAL,
  grader_comment text,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_audio_plays (               -- §11.4
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  plays       integer NOT NULL DEFAULT 0 CHECK (plays >= 0),
  last_played_at timestamptz,
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_events (                    -- append-only, §10
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  kind        text NOT NULL,
  occurred_at timestamptz NOT NULL,                  -- client time, offset-corrected
  received_at timestamptz NOT NULL DEFAULT now(),
  client_seq  integer NOT NULL,
  question_id uuid,
  meta        jsonb,
  UNIQUE (attempt_id, client_seq)
);
CREATE INDEX ON app.attempt_events (attempt_id, occurred_at);
```

`final_score` uses PG18 **virtual generated columns** — computed on read, no storage, never stale (the default for generated columns in 18). Grading precedence is a pure function of two columns and must never drift.

`attempt_events` uses `bigint IDENTITY` rather than a UUID: a high-volume append-only log read only by `attempt_id` is better served by a narrow sequential key. The application never updates or deletes it. An explicit privileged retention operation may delete events only after both assignment closure and server receipt are more than thirteen calendar months old, using server `received_at` and a UTC cutoff; each batch is audited. The audit log remains retained. Disabled accounts are not automatically erased. Manual requested student anonymization removes structured identity and credentials while preserving historical IDs and records; free text, old audit entries and backups require separate review (see `docs/setup/operations.md`).

### 13.4 Audit log

`audit_log(id, actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff jsonb)`. Required entries: class enrolment (§6.5), join-code generation and rotation, attempt reset/void/extend, password reset, media deletion, test publish.

Use PG18's `OLD`/`NEW` in `RETURNING` to capture the diff in the same statement as the update, rather than a read-then-write.

**Reading the log (R5).** `GET /admin/audit-log` and `GET /admin/audit-log/export` (`system.audit.read`, Admin only) read it through `modules/audit`; no other module reads `audit_log`. An entry's category comes from its `entity` (`domain/categories.go`: six tabs; organization, export, maintenance, retention and AI entries are listed under All only), and its sentence from its `action` (`domain/actions.go`, `vi` and `en`, matched by the same `q` that matches the actor's name or email and the name of the row the entry is about). The tab counts are one `GROUP BY entity` over the search and range, whichever tab is selected. The ranges are calendar days in the viewer's stored zone (`7d` is today and the six days before it). A diff shaped `{field: {old, new}}` is shown as `changes` and any other as `details`; a `code`, `joinCode` or `code_hash` value is shown as its last four characters and a key whose name contains `password`, `passcode`, `token`, `secret`, `authorization`, `credential`, `cookie`, `ciphertext` or the name of a key (`apiKey`, `privateKey`, `accessKey`, `signingKey`), or is exactly `otp` or `pin`, is dropped, whatever was stored. The export streams CSV or JSON in keyset batches over `audit_log_recent_idx`, refuses more than 100,000 entries with `422 EXPORT_TOO_LARGE`, reads the log as it stood when it began, and is itself audited as `audit.exported` before its first byte. A search runs with a 30-second statement timeout and `jit` off, because the planner's cost for its sub-selects otherwise starts JIT compilation that costs a second. A change to what the server writes (a new `action` or `entity`) fails `modules/audit/domain/tests/catalog_test.go` until `domain/actions.go` or `categories.go` and both locale files list it.

### 13.5 Security in the data layer

- Refresh tokens stored as SHA-256 hashes: `token_hash`, `family_id`, `user_id`, `expires_at`, `revoked_at`, `replaced_by`, `user_agent`, `ip`, `geo_label` (the place of the request that issued the token, "City, CC", or NULL; R4).
- Join codes sealed with AES-256-GCM under a key the database never holds, and found by an HMAC-SHA256 of the normalized code under a key derived from it (§13.3). Lookup by hash, compared in constant time. A live code issued before v0.8.0 kept its SHA-256 hash until v0.10.0's start-up rotation replaced it (§6.1); a revoked or expired one keeps it.
- Passwords: Argon2id (bcrypt cost ≥ 12 if unavailable).
- `sample_answer`, `transcript`, `is_correct`, and `accepted_answers` must never reach a student response. Enforce with explicit column lists — **no `SELECT *` in student-facing paths.** Add a test that asserts these keys are absent from `GET /app/attempts/:id`.
- Least-privilege DB roles: the app connects with DML on `app` only, not as owner. Migrations run as a separate role. The app role only reads `permissions` and `student_like_roles`; on `roles` and `role_permissions` it holds only what the role commands write (`00129_grant_role_writes.sql`: insert roles, update their name, description, icon, colour, revision and `updated_at`, add and remove grants), never `DELETE` on `roles`. `users_last_admin` is the schema's first `SECURITY DEFINER` function: owned by the migrate role, with `search_path = pg_catalog, app, pg_temp` and EXECUTE revoked from PUBLIC (`docs/plan/20-data-model.md` §30).
- `app.organization_settings` is one row (`id boolean PRIMARY KEY DEFAULT true CHECK (id)`, migration `00130`): the app role may `SELECT` and `UPDATE` it and has no `INSERT` or `DELETE` (§5.6).

### 13.6 PG18 features — use and skip

| Feature | Use? | Why |
|---|---|---|
| `uuidv7()` | **Yes**, all PKs | Time-ordered → index locality; avoids v4 fragmentation |
| Virtual generated columns | **Yes**, `final_score` | Computed on read, no storage, never stale |
| `OLD`/`NEW` in `RETURNING` | **Yes**, audit log | One statement instead of read-then-write |
| B-tree skip scan | Passive | One multicolumn index serves more shapes — do not add redundant prefix indexes preemptively |
| `NOT NULL NOT VALID` + `VALIDATE` | **Yes**, in migrations | Adds `NOT NULL` without a full-table lock |
| Async I/O | Passive | Server-side; no schema impact |
| Temporal constraints (`WITHOUT OVERLAPS`) | **No** in v1 | No business rule needs non-overlapping validity periods. Revisit for class scheduling. |
| OAuth DB authentication | **No** | That is Postgres *connection* auth, unrelated to §5's application-level Google Sign-In. Do not confuse the two. |

### 13.7 Migrations

- **goose**, SQL, forward-only, one concern per migration.
- Expand-contract for anything breaking: add nullable → backfill → start writing → `NOT NULL` → drop old. Never in one migration.
- `CREATE INDEX CONCURRENTLY` runs outside a transaction — mark those `-- +goose NO TRANSACTION`.
- Test every migration against a copy of real data before production. On Neon, a branch per migration, reset from parent between runs.
- Seed data in `seed/`, never in migrations. The exception is reference data the app cannot run without: the permission catalogue and the built-in roles (`docs/plan/20-data-model.md` D-21).
- A migration that only rewrites data has a Down that does nothing, and says so. `00103_compose_text_nfc.sql` composes the plain text columns a person types to NFC (§12) without moving `updated_at`; nothing records which rows were decomposed, so nothing can be restored, and NFC and NFD are the same text to a reader (`docs/plan/20-data-model.md` D-38).

### 13.8 Query discipline

- No `SELECT *` in application code.
- Keyset pagination everywhere (`WHERE (created_at, id) < ($1,$2) ORDER BY created_at DESC, id DESC LIMIT $3`), not `OFFSET`. `uuidv7()` PKs make this natural.
- `EXPLAIN (ANALYZE, BUFFERS)` any query touching `attempts`, `attempt_answers`, or `attempt_events` before merging.
- N+1 is the default failure mode of the grading and monitor screens — one query for an attempt's answers, one query for an assignment's monitor rows.

**The Admin's readings (R5).** `getAdminOverview`, `getAdminSummary` and `getUsage` (`modules/dashboard`, all read-only, no migration) count the whole centre under no scope: the caller's keys decide which sections are read at all, and a section a key does not open is `null` and costs no query. A day is a local date in `APP_TIMEZONE`: the chart is today and the thirteen days before it with a zero for a day with no row in `user_activity_days`, its average the mean of students plus staff rounded half up, and "this week" the seven days ending today. A student is a student target of `app.student_like_roles` and a teacher is every other account, an Admin and a custom role included; the chart's "staff" is the same partition. `people.teachers` and `usage.teachers` count the enabled teachers, `people.teachersDisabled` the disabled ones, and a seat is an account that is not disabled; `people.total` and `people.students` still count disabled accounts, as the Users tabs do. The enabled accounts fall into the first of must change password, never signed in (DG-54). The plan is `PLAN_NAME`, `PLAN_RENEWS_ON`, `PLAN_SEAT_LIMIT`, `PLAN_TEACHER_LIMIT`, `PLAN_MEDIA_BYTES` and `PLAN_IMPORT_BYTES`, each optional, validated at start-up and never enforced (DG-55); a meter at 80% of its limit adds the Overview's storage row. The AI figures, the credit-request row and `aiCredits`, come through the AI module's own `AdminCredits` query for a holder of `ai.credits.manage` and are null while the assistant is off. `attention[].detail` carries the facts a row quotes, not a sentence, so the web words each row in the reader's language.

### 13.9 Maintenance windows

`app.maintenance_windows (id, starts_at, ends_at, created_at, created_by, cancelled_at)` (migration `00053`, `docs/plan/20-data-model.md` §27).

- A window runs at most 12 hours, and windows that are not cancelled may not overlap (an exclusion constraint).
- The app role only reads the table. The operator schedules, cancels and ends windows through `cmd/maintenance`.
- Scheduling extends by the window's length every in-progress attempt that would still be running when it starts, every published, open assignment that would close inside it, and every student override of a published assignment that would close inside it. Every extension is audited as System.
- The API reads the table into a snapshot at most every 30 s, and only while requests arrive. During an active window it answers 503 (§15), and it refuses an attempt start that would run into a window.
- A start and a schedule take the same advisory lock (73819, 40), so a new attempt is either refused or extended. So do every other writer of an assignment's window or overrides: the teacher's extension, the assignment update, the reopening and the two override writes (shared, as a start does), so none of them runs between a window's scheduling and its extensions.

### 13.10 AI credits

The assistant's budget (D21) is six tables, migrations `00118` to `00123` (`docs/plan/20-data-model.md` §46). All are new and empty, so a previous binary is unaffected, and they exist whether or not `ANTHROPIC_API_KEY` is set. A credit is $0.001 of the provider's list price: a call costs `ceil(sum(tokens × price) / 1e9)` from the provider's reported usage and the tier's configured prices, held as integer millionths of a dollar per million tokens, so no call is rounded across a credit by floating point. The centre has a pool of 200,000 credits a month ($200) and every teacher an allowance under it, 20,000 by default.

| Table | Holds |
|---|---|
| `ai_settings` | One row, written by its migration: the pool the plan sets, the default allowance, the share of an allowance at which a teacher is told (80) and the rule for a teacher who runs out, `request` or `pause`. The pool has one rule for running out, pause everyone (DG-196). |
| `ai_credit_periods` | One row a month, created by the month's first use from `ai_settings`: `pool_credits`, `added_credits` (an Admin's approval, not carried over), `used_credits`, `reserved_credits`. |
| `ai_teacher_credits` | One row per teacher and month: `allowance_credits` (with the `granted_credits` that approved requests added), `custom`, `paused`, `used_credits`, `reserved_credits`, `warned_at`. `custom` and `paused` follow the person into the next month. |
| `ai_reservations` | The ceiling a call holds until it settles or is released. A leftover older than ten minutes is returned by `ReleaseStaleAiReservations` (hourly, and `maintenance ai-reservations-sweep`). |
| `ai_usage_ledger` | One row per settled call: the actor, the feature, the tier, the four token counts the provider reported, the credits charged and the provider's request id. |
| `ai_credit_requests` | A teacher's request for more credits, `pending`, `approved` or `declined`, with who decided and when. A partial unique index allows one pending request per teacher and month. |

- **The hard stop is two constraints.** `ai_credit_periods_within_pool` and `ai_teacher_credits_within_allowance` each say `used + reserved <= total`, and every credit moves in an `UPDATE` of those rows. The database refuses the call that would overspend, and no code path has to remember to. A reservation locks the month's row and then the teacher's, in that order everywhere, so two calls cannot deadlock. A settlement charges the least of the metered credits and what was reserved, so a call the provider has been paid for is never refused.
- **A month** is the calendar month in `APP_TIMEZONE` (`Asia/Ho_Chi_Minh` when unset). The pool, the allowances and the use start over on the 1st; credits an Admin adds do not carry over.
- **`ai_usage_ledger` is append-only**, as `audit_log` and `attempt_events` are: the application role has no `UPDATE` or `DELETE` on it, so no handler can rewrite what was spent. Its `user_id` is set to NULL by the foreign key when a person is deleted, so a row outlives its actor.
- **Every Admin write is one `audit_log` row**, from a data-modifying CTE, so the diff cannot disagree with the change. A write from `cmd/maintenance ai-credits` is System: no actor, `reason: "maintenance"` and the operator in `details`.
- **Two permission keys** (migration `00124`): `ai.use` in Content, granted to the Teacher role, and `ai.credits.manage` in System, granted to no built-in role; the Admin's wildcard holds both (`docs/plan/70-redesign-overview.md` §4.1). The import's `word_imports.read_with_ai` (migration `00125`, `boolean NOT NULL DEFAULT false`) is §16's.

---

## 14. Testing & definition of done

**Unit (Vitest):** zod schemas, utils, take-test store (timer math, resume merge, submit idempotency), integrity buffering and strike counting, join-code normalization, audio plays-remaining reconciliation.

**Component (Testing Library):** each form validates and submits; guards redirect correctly; a renderer test per `QuestionType`; integrity dialog states; audio player states (idle / playing / plays exhausted / seek blocked / load error).

**E2E (Playwright + MSW or a seeded backend):**
1. Admin logs in → creates a test with one of each question type **including audio** → publishes → assigns.
2. Student logs in with **password** → starts → answers → reloads mid-test → answers persist → submits → sees result.
3. **Join flow:** anonymous visitor opens `/join/:code` → sees class name → Google sign-in (mocked GIS) → account created, enrolled, lands on `/app`.
4. **Expired code** → plain error, no account created, nothing leaked about the class.
5. Timer expiry auto-submits.
6. Tab-switch fires `tab_hidden`, warning dialog appears, event lands on the admin integrity timeline.
7. Second tab on the same attempt supersedes the first; the first goes read-only.
8. **Audio `maxPlays`:** play twice → play button disabled → **reload** → still disabled (server-authoritative).
9. Student payload assertion: `GET /app/attempts/:id` contains no `isCorrect`, `sampleAnswer`, `transcript`, or `acceptedAnswers`.

**Definition of done for any task:**
- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] All strings via `t()`, keys in both `vi` and `en`
- [ ] Keyboard-operable
- [ ] Tests added/updated per the levels above
- [ ] Any DDL reviewed against §13 and the Neon skill
- [ ] Any new public (unauthenticated) endpoint is rate-limited and leaks nothing (§6.5)
- [ ] No new dependency without a stated reason in the PR description

---

## 15. Assumed API surface

Base `VITE_API_BASE_URL`, JSON, `Authorization: Bearer <access>`.

```
# public (unauthenticated — rate-limited, §6.5)
GET    /public/status                   → {maintenance: {startsAt, endsAt, active} | null, ai}
                                          same answer for everyone, Cache-Control public 30 s,
                                          120/min and 2,000/h per IP; ai is true when the AI
                                          assistant is configured, read from configuration and
                                          never from the provider
GET    /public/organization             -> {name, shortName, website, contactEmail, frontDeskPhone, frontDeskHours, address,
                                          logo {lightUrl, darkUrl}, signIn {google, password}, joinCodes}; same body for
                                          everyone, Cache-Control public 300 s, 120/min and 2,000/h per IP
GET    /public/organization/logo        ?variant=light|dark&v=<sha256> -> the logo bytes, nosniff, a PNG attachment; 404
                                          when there is none; 300/min per IP
POST   /join/preview                    {joinCode} → {classId, className, teacherName}
POST   /auth/login                      {email,password} → {accessToken,user: CurrentUser}
POST   /auth/google                     {code,codeVerifier,redirectUri,joinCode?} → {accessToken,user: CurrentUser}
POST   /auth/refresh                    (cookie) → {accessToken}
POST   /auth/set-password/check         {token} → {email,fullName,purpose: invite|reset,expiresAt}; 410
                                          SET_PASSWORD_LINK_INVALID for every link that cannot be used (§5.4);
                                          120/min and 1,000/h per address, 20/h per token
POST   /auth/set-password               {token,newPassword} → {accessToken,user: CurrentUser} and the refresh
                                          cookie; 410 SET_PASSWORD_LINK_INVALID, 422 PASSWORD_SIGN_IN_DISABLED,
                                          400 VALIDATION_FAILED on the password rules; 60/min and 600/h per
                                          address, 10/h per token

# authenticated
POST   /auth/logout
GET    /auth/me                         → CurrentUser
PATCH  /auth/me                         {fullName?,displayName?|null,phone?|null,locale?,timeZone?}
                                          → CurrentUser; at least one supplied field;
                                          omitted fields unchanged; unknown zone400 VALIDATION_FAILED
PATCH  /me/preferences                 top-level UserPreferences merge → UserPreferences;
                                          nested assignmentDefaults and ai {language, tagDrafts,
                                          readStudentAnswers, readImports} replace their key, {} no-op;
                                          raw/stored UTF8 cap8192 bytes,400 on excess
GET    /me/notifications?before=&limit=&audience= → {items: [Notification], nextBefore}; the caller's own,
                                          newest first (20 by default, 50 at most), of one console's
                                          kinds when audience is teacher or student. Writes the caller's
                                          due-time notifications first, at most once in 5 minutes
POST   /me/notifications/read           {ids?, audience?} (1 to 100; absent means all; audience narrows
                                          both to that console's kinds) → 204 whatever matched
GET    /me/summary?audience=            → {unreadNotifications}, of one console's kinds when audience
                                          is given; writes the due-time notifications first
GET    /me/notification-preferences     → the five switches [{event,inApp,email}]
PUT    /me/notification-preferences     the five switches, each once → the stored switches
PUT    /me/avatar                      multipart file (PNG or JPEG, at most 2 MiB, each side 200–2048 px,
                                          a JPEG at most 32 scans) → CurrentUser; stored as a 256×256 PNG,
                                          EXIF orientation applied then dropped; 413 MEDIA_TOO_LARGE, 415
                                          MEDIA_TYPE_UNSUPPORTED | MEDIA_UNREADABLE | IMAGE_DIMENSIONS;
                                          10 an hour per user
DELETE /me/avatar                      → CurrentUser without avatarUrl
POST   /auth/change-password            400 VALIDATION_FAILED on the rules (§5.4), 400 PASSWORD_UNCHANGED
POST   /auth/set-password/request       → 204: emails the caller a set-password link and keeps their session;
                                          401 for a disabled account, 422 PASSWORD_SIGN_IN_DISABLED, 501 when
                                          the server cannot send one; 3/min and 10/h per user
POST   /auth/google/link                link Google to current account → CurrentUser
DELETE /auth/google/link                rejected if it would leave no login method
GET    /auth/sessions                   → {items: [Session]}: the caller's live sessions, at most 100, the one the
                                          refresh cookie names first; {familyId, device|null, deviceKind, location|null,
                                          lastUsedAt, current}. Without a cookie that names one, none is current
DELETE /auth/sessions/{familyId}        → 204; 404 for another user's, revoked, expired or unknown id, 409 SESSION_IS_CURRENT
                                          for the calling session, 401 when no cookie names the caller's session;
                                          moves the session epoch; 10/min and 60/h per user
POST   /auth/sessions/revoke-others     → {revoked}; 401 as above; moves the epoch only when revoked > 0;
                                          5/min and 30/h per user

# teacher
GET    /teacher/dashboard?range=7d|14d|30d → Dashboard (default 14d; own teaching)
GET    /teacher/summary?audience=      → TeacherSummary {liveAssignments,answersToGrade,unreadNotifications};
                                          the count is of one console's kinds when audience is given
GET    /teacher/tests?status=&tag=&q=&page=&limit=
POST   /teacher/tests | GET /:id | PATCH /:id
POST   /teacher/tests/:id/publish       {changeNote?} (at most 200 characters; blank or null is none) → 201 TestVersion;
                                          its number is the `nextVersion` the test reported, and the answer alone
                                          carries testUpdatedAt, the test's updatedAt after the publish, which a
                                          caller that goes on saving sends as expectedUpdatedAt or gets STALE_WRITE
GET    /teacher/tests/:id/versions      → the versions, newest first: {version, totalPoints, questionCount, audioCount,
                                          manualCount, skills, publishedAt, publishedBy, assignmentCount, changeNote}
GET    /teacher/tests/:id/versions/:version/diff?against=previous|draft|<n>
                                        → {from, to, changes}; reads only, 400 for the version itself, 404 for a missing
                                          version or another teacher's test, 422 when a group cannot be read back
POST   /teacher/tests/:id/duplicate
POST   /teacher/tests/:id/questions/move  {expectedUpdatedAt, questionId, to: {sectionId, groupId|null, index},
                                          expectedGroupRevisions: [{groupId, revision}]} (one or two, once each,
                                          exactly the groups the move touches) → {test, questionId, copied, restored, editedCopy,
                                          groups};
                                          content.tests.write, at most 4 KiB. index counts once the question has
                                          left its old place; past the end is the end. A standalone question to a
                                          group is copied (copied true, a new questionId, which remembers the bank
                                          question); a member out of a group goes back as the original when it is an
                                          unedited copy of a bank question still in the bank (restored true,
                                          questionId the original's), otherwise keeps its id (editedCopy true when it
                                          was an edited copy of a question still there); a member to another group or
                                          within one keeps its id. A move between two standalone
                                          places is updateTest's. 400 VALIDATION_FAILED with details.rule
                                          move_outline_only | move_group_section | move_revisions |
                                          move_source_ambiguous; 404 for a test, question, section or group the
                                          caller cannot reach or that is not of this test, an expectedGroupRevisions
                                          entry included, before any revision is compared; 409 STALE_WRITE (the test or
                                          a touched group is behind, or a touched group is missing from the list) |
                                          TEST_ARCHIVED | GROUP_CONFLICT; 413 over 4 KiB; 422 VALIDATION_FAILED with
                                          details.rule group_gap_bound (and stimulusId, gapId, questionId) |
                                          group_limits | group_audio_scope | group_asset | group_reference.
                                          Nothing is written on a refusal
GET    /teacher/questions?type=&tag=&tagMatch=any|all&level=&skill=&hasAudio=&q=&page=&limit=
POST   /teacher/questions | PATCH /:id | DELETE /:id
                                        an audio policy carries allowPause (§11.1): optional in a request,
                                        where a create stores true and an update keeps the stored value;
                                        always sent in an answer. The group writes follow the same rule
POST   /teacher/media?defaultMaxPlays=  multipart → MediaAsset (validates mime, size, duration, quota)
GET    /teacher/media?kind=&unused=&q=&page=&limit=   rows, totalBytes, facets and usage (library, held and pending bytes) against the quota
PATCH  /teacher/media/:id               {displayName?, defaultMaxPlays?} → the library row
POST   /teacher/media/:id/replace       multipart → {asset, repointed: {questions, groups}, left: {questions, groups}, previous};
                                          415 MEDIA_KIND_MISMATCH for a file of the other kind (§11.1); 429 past ten a minute
DELETE /teacher/media/:id               409 if referenced by a published version; 200 → {id, bytes, url, usedIn, deleteAt}
GET    /teacher/assignments?status=&classId=&classId=&q=&page=&limit= | POST | GET /:id | PATCH /:id
                                        rows carry questionCount; classId repeats (any of them)
                                        PATCH 409 ASSIGNMENT_LOCKED while the assignment is open and the
                                        body changes testVersionId, durationMinutes or maxAttempts;
                                        PATCH 409 ASSIGNMENT_CLOSED when the assignment is closed and
                                        the window would open it again (use /reopen);
                                        PATCH 400 VALIDATION_FAILED naming closeNow when closeNow is
                                        sent with opensAt still ahead (a POST ignores closeNow)
POST   /teacher/assignments/:id/extend  {minutes,notify?} → Assignment; 409 ASSIGNMENT_CLOSED when closed
POST   /teacher/assignments/:id/duplicate {classIds} → 201 Assignment, a draft; 409 TEST_NOT_PUBLISHED
GET    /teacher/assignments/:id/item-analysis → {handedIn, items:[{questionId,number,type,promptExcerpt,
                                          answered,correctRate|null}]}, hardest first
GET    /teacher/assignments/results.csv?ids=  → text/csv, BOM; 404 for any id not reached; 422 over 20000 rows
GET    /teacher/assignments/:id/student-overrides   → {items}
PUT    /teacher/assignments/:id/student-overrides   {studentIds,extendBy?|closesAt?,durationMinutes?,
                                          extraAttempts?,reason,notify?} → {items}; 422 for a student
                                          who is not a target the caller reaches; 409 ASSIGNMENT_CLOSED
                                          for extendBy on a student whose close has passed
DELETE /teacher/assignments/:id/student-overrides/:studentId
GET    /teacher/assignments/:id/attempts  → rows incl. integrity + audio summary; a row carries extendedTo
                                          when the student's own override closes later than the assignment's
                                          effective close, and not otherwise
POST   /teacher/attempts/:id/extend     {minutes,reason}
POST   /teacher/attempts/:id/reset      {reason}
POST   /teacher/attempts/:id/void       {reason}
GET    /teacher/attempts/:id | GET /teacher/attempts/:id/events
POST   /teacher/attempts/:id/grade      {items:[{questionId,points,comment}]}
POST   /teacher/attempts/:id/finish-grading
GET    /teacher/grading/queue?mode=student|question&assignmentId=&studentId=
                                          → {groups,items,answersRemaining,studentsWaiting}; default student
GET    /teacher/students?q=&classId=&classId=&status=&mustChangePassword=&page=&limit= | POST | GET /:id
PATCH  /teacher/students/:id            403 FORBIDDEN when disabled is sent without
                                          people.users.manage or the subset rule refuses; 403 STUDENT_SHARED
                                          for a new email when someone else also reaches the student (§5.4)
POST   /teacher/students/reset-passwords {studentIds (1-40)}
                                          → 200 {items:[{studentId,fullName,email,temporaryPassword}],
                                          failed:[{studentId,code}]}, each in the order asked, Cache-Control
                                          no-store; code NOT_FOUND | FORBIDDEN | STUDENT_SHARED | INTERNAL (§5.4)
POST   /teacher/students/:id/reset-password
                                          403 STUDENT_SHARED when someone else also reaches the student,
                                          403 FORBIDDEN when the subset rule refuses (§5.4)
GET    /teacher/classes?q=&page=&limit=&status=&withCodes=
                                          rows carry joinCode in full, its code opened only with
                                          withCodes=true for a holder of teaching.classes.write and
                                          only where getJoinCode would open it; Cache-Control no-store
POST   /teacher/classes                 {name, description, scheduleLabel, room, selfJoinEnabled}
GET    /teacher/classes/:id
PATCH  /teacher/classes/:id             {name, description, scheduleLabel, room, selfJoinEnabled, archived};
                                          an omitted scheduleLabel or room keeps its value, null or blank clears it
DELETE /teacher/classes/:id             archived only, else 409 RESOURCE_NOT_ARCHIVED;
                                          409 RESOURCE_REFERENCED {referencedBy}
POST   /teacher/classes/:id/members | DELETE /teacher/classes/:id/members/:userId
GET    /teacher/classes/:id/join-code   → {code, hint, legacy, expiresAt, maxUses, usesCount}
                                          Cache-Control no-store; code null for a legacy code or one
                                          sealed under a key the server no longer holds; 404 without
                                          an active code
POST   /teacher/classes/:id/join-code   rotate {expiresInDays, maxUses} → {code, expiresAt, maxUses};
                                          expiresAt is 23:59:59 of the last day in the caller's zone (§6.1);
                                          409 CLASS_ARCHIVED for an archived class, nothing written (§6.4)
DELETE /teacher/classes/:id/join-code   revoke, disable self-join

# teacher: Word imports (content.tests.write; the caller's own imports, §16)
GET    /teacher/imports/capabilities    → {intakeEnabled, processingEnabled, retention}
GET    /teacher/imports/limits          → {maxBytes, formats, pasteMaxCharacters}
POST   /teacher/imports                 {requestId, title, readWithAi?} → 201 WordImport, idempotent on requestId;
                                          422 VALIDATION_FAILED naming readWithAi when it is true and the server
                                          has no assistant or the caller lacks ai.use (§16)
GET    /teacher/imports?status=&status=&q=&page=&limit= → {items: [WordImportHistoryItem], facets}; every item
                                          carries reviewCounts {needsAction, toConfirm} or null
GET    /teacher/imports/:id             → WordImport with its current sources
POST   /teacher/imports/:id/sources     multipart .docx | .doc | .pdf → 201 ImportUploadReceipt
POST   /teacher/imports/:id/sources/text {uploadId, expectedRevision, text} → 201 ImportUploadReceipt (§16);
                                          400 over 100000 code points as sent, 413 over 100000 after NFC or 20000
                                          non-blank lines, 415 for a NUL or invalid UTF-8, 409 IMPORT_CONFLICT
GET    /teacher/imports/:id/sources/:sourceId/download → a 60-second URL; 410 IMPORT_FILES_REMOVED
POST   /teacher/imports/:id/process     → 202; 503 IMPORT_PROCESSING_UNAVAILABLE without a worker
POST   /teacher/imports/:id/cancel
GET    /teacher/imports/:id/review | PUT | POST /:id/review/adopt   the draft, the teacher's save, a newer machine draft
GET    /teacher/imports/:id/source?role= → one source's extracted blocks
POST   /teacher/imports/:id/commit      → one draft test

# teacher: AI assistant (ai.use; a suggestion only, nothing is saved; 501 AI_OFF without a provider)
POST   /teacher/ai/questions/generate   {source | testId + stimulusId, types (the five stored), count 3|5|8,
                                          level b1|b2|c1, explanations, language}
                                          → {items: [{type, prompt, options?, answerKey, explanation}], usage}
POST   /teacher/ai/tests/:id/check      {checks ⊆ key|clarity|giveaway|language, language?} → {findings: [{severity,
                                          questionId, number, type, title, issue, fix: {kind, description, label?,
                                          payload?}}], usage}; 422 for a draft with no question or over 120,000 characters
POST   /teacher/ai/tests/:id/variant    {changes ⊆ reword|reorder|distract, difficulty same|easier|harder} → {changes:
                                          [{questionId, number, kind, title, before, after, payload}], usage}; 422 as check
POST   /teacher/ai/questions/:id/explanation {language, length?} → {text, usage}; 422 without an answer key
POST   /teacher/ai/questions/:id/distractors → {items: [{text, why}] (up to 4), usage}; 422 unless a single or
                                          multiple choice question with a key
POST   /teacher/ai/attempts/:attemptId/answers/:questionId/score-suggestion
                                        → {score (0 to the points, steps of 0.5), confidence, reasons, comment, usage};
                                          also needs teaching.grading and the caller's readStudentAnswers (403
                                          AI_READ_OFF); 409 ATTEMPT_IN_PROGRESS | ATTEMPT_VOIDED; 422 for an answer the
                                          grader marked itself or the student left empty
POST   /teacher/ai/assignments/:id/insights → {basis, items: [{kind, title, body, action, questionNumbers, types}]
                                          (up to 4), usage}; the same gates; 422 when no paper is handed in
POST   /teacher/ai/bank/cleanup         {checks ⊆ duplicates|tags, language?} → {scanned, truncated, duplicates, tags,
                                          usage} over the caller's own questions only, even for scope.all; 3 an hour
GET    /teacher/ai/usage                → {month, used, allowance, left, paused, requests, pending, categories}
POST   /teacher/ai/credit-requests      {amount 1,000..100,000, reason} → 201 AiCreditRequest; 409 AI_REQUESTS_OFF |
                                          AI_REQUEST_PENDING; 5 an hour

# admin: roles (people.roles.manage unless noted)
GET    /admin/roles                     → {roles: [{id, builtinKey|null, name, description, icon, color, custom, userCount,
                                          permissions (without the hidden keys), revision, editable, inProgressAttempts}],
                                          catalogue: [{key, group, adminLocked}]}
POST   /admin/roles                     {name 1..60, description ≤300, copyFromRoleId?} → 201 RoleSummary (icon key-round,
                                          colour gray); 403 TARGET_ROLE_NOT_ALLOWED, 409 ROLE_NAME_TAKEN,
                                          422 ROLE_NOT_ASSIGNABLE (the Assistant as the source)
PATCH  /admin/roles/:id                 {name?, description?, icon?, color?, expectedRevision} → RoleSummary; 403
                                          TARGET_ROLE_NOT_ALLOWED, 409 STALE_WRITE | ROLE_NAME_TAKEN
PUT    /admin/roles/permissions         {changes: [{roleId, key, granted}] (1..200), expectedRevisions} → {roles}; one
                                          transaction, refused whole: 403 TARGET_ROLE_NOT_ALLOWED, 409 STALE_WRITE,
                                          422 ROLE_LOCKED | STUDENT_NEEDS_TAKE_TESTS | KEY_NOT_GRANTABLE | CANNOT_ACT_ON_SELF
GET    /admin/roles/assignable          (people.users.manage) → [{id, builtinKey|null, name, icon, color, studentTarget}]:
                                          roles within the caller's own permissions, without the Assistant

# admin: data export (system.data_export, Admin only; 501 EXPORT_OFF without import storage and the worker)
POST   /admin/data-exports              {includeAnswers} → 202 DataExport {id, status queued|running|ready|failed|expired,
                                          includeAnswers, bytes, createdAt, completedAt, expiresAt, downloadedAt, downloads 0..3}; 409
                                          EXPORT_IN_PROGRESS while any export is queued or running; 429
                                          RATE_LIMITED + Retry-After while the caller has an export from the last hour that
                                          has not failed (a failed export and a refused request do not count); 10 a minute per actor
GET    /admin/data-exports/:id          → DataExport; another admin's export is 404; a ready export past expiresAt reads expired
POST   /admin/data-exports/:id/download → {url, expiresInSeconds: 60}: a presigned attachment, `quizzivy-export-YYYYMMDD.zip`,
                                          up to three times, by the requester (each signing audited, DataExport.downloads
                                          counts them); 404 for another admin's export, 410 EXPORT_UNAVAILABLE unless it is
                                          ready, unexpired and signed fewer than three times; 10 an hour per actor

# admin
GET    /admin/users                     (people.users.manage) ?tab=all|teachers|students|admins|disabled
                                          &status=must_change_password|never_signed_in &q &email (exact)
                                          &page &limit (10, 20 or 50) → {items: AdminUserRow[], page, facets}
GET    /admin/users/:id                 (people.users.manage) → {user, createdBy|null, classes (≤ 20), ownedContent,
                                          inProgressAttempts, activity (≤ 8), can}; 404
POST   /admin/users                     (people.users.manage) {fullName, email, roleId, classId?, delivery: email_link |
                                          temporary_password} → 201 {user: AdminUserRow, temporaryPassword?} with
                                          Cache-Control: no-store; 403 TARGET_ROLE_NOT_ALLOWED, 404, 409 EMAIL_TAKEN,
                                          422 ROLE_NOT_ASSIGNABLE | PASSWORD_SIGN_IN_DISABLED
DELETE /admin/users/:id                 a disabled student only, else 409 RESOURCE_NOT_ARCHIVED;
                                          403 FORBIDDEN when the subset rule refuses,
                                          409 RESOURCE_REFERENCED {referencedBy}
POST   /admin/docs-session              opens the API reference for fifteen minutes (§5.5)
POST   /admin/user-imports/preview      (people.users.manage) multipart {file, defaultRole: student | teacher,
                                          delivery: email_link | sheet} → {rows, newAccounts, existingEmails {count,
                                          sample ≤ 5}, invalid [{row, field, code}] ≤ 50, invalidCount, classes
                                          [{id, name}], emailBudget {remainingToday, limit} | null}; writes nothing;
                                          10 a minute per actor; 403 TARGET_ROLE_NOT_ALLOWED, 413
                                          USER_IMPORT_FILE_TOO_LARGE, 422 USER_IMPORT_* | CSV_NOT_UTF8 | XLSX_MACROS |
                                          XLSX_ENCRYPTED | PASSWORD_SIGN_IN_DISABLED, 501
POST   /admin/user-imports              (people.users.manage) the same plus requestId → 202 UserImportJob {id, status,
                                          total, processed, created, skipped, invalid, delivery, sheet | null, errorCode?};
                                          the same requestId returns the same job; 5 an hour per actor; 422
                                          USER_IMPORT_NOTHING_TO_CREATE besides the preview's codes
GET    /admin/user-imports/:id          (people.users.manage) → UserImportJob; another user's job is 404
POST   /admin/user-imports/:id/sheet    (people.users.manage) the importer only → text/csv attachment, no-store,
                                          full_name,email,role,class,temporary_password; once: 410 SHEET_UNAVAILABLE
                                          afterwards, for an email job, before the job is done and after 24 hours;
                                          404 for another user's job; 5 a minute per actor; 501

# admin: organization settings (system.settings.write)
GET    /admin/settings                  -> OrganizationSettings {organization, signIn, retention, revision, updatedAt, updatedBy}
PUT    /admin/settings                  {organization, signIn, retention, expectedRevision} -> OrganizationSettings; 400
                                          VALIDATION_FAILED per field | 409 STALE_WRITE | 422 SIGN_IN_POLICY_LOCKOUT; a save
                                          that changes nothing keeps the revision and writes no audit row
POST   /admin/settings/sign-in-impact   {signIn} -> {affectedUsers, affectedAdmins, activeAdmins, actorLocked}; writes nothing
POST   /admin/settings/logo             multipart {file, variant light|dark} -> {sha256, type, width, height}; 413 | 415
                                          MEDIA_TYPE_UNSUPPORTED (not a PNG) | MEDIA_UNREADABLE | IMAGE_DIMENSIONS; 20 an hour

# admin: readings (R5, T-R5.18); a section is null without its key
GET    /admin/overview                  (workspace.admin) → {organizationName, people|null, classes|null, dailyActive|null,
                                          attention, recentChanges|null, storage|null}: people and dailyActive need
                                          people.users.manage, classes scope.all, recentChanges system.audit.read, storage
                                          system.settings.write; attention rows with a count of zero are left out
GET    /admin/summary                   (workspace.admin) → {mustChangePassword|null}: null without people.users.manage
GET    /admin/usage                     (system.settings.write) → {plan|null, seats, teachers, media, imports, attemptsKept,
                                          aiCredits|null}: each limit is null when its PLAN_* variable is unset

# admin: AI credits (ai.credits.manage)
GET    /admin/ai/credits                → {month, pool, daysToGo, onPace, withinPool, allowances, active, idle,
                                          teachers, requests, rules}
PATCH  /admin/ai/rules                  {defaultAllowance?, warnPercent? 50..99, onTeacherLimit? pause|request}
                                          → {rules, rewritten}
POST   /admin/ai/credits/additions      {credits} → 201 AiPool; the approval to spend beyond the plan
PUT    /admin/ai/teachers/:userId/allowance {custom?, credits?, paused?} → AiTeacherMonth; 409
                                          AI_ALLOWANCE_BELOW_USE; the subset rule applies (FORBIDDEN)
POST   /admin/ai/credit-requests/:id/approve | /decline → {request, allowance}; 409 AI_REQUEST_DECIDED |
                                          AI_REQUEST_EXPIRED; the subset rule applies

# student
POST   /app/classes/join                {joinCode} → Class      (already-authed path)
GET    /app/classes
GET    /app/assignments                 → {dueNow,upcoming,completed}; a card with a live attempt
                                          carries liveAnsweredCount
GET    /app/assignments/:id
POST   /app/assignments/:id/attempts    → create or resume → Attempt + ordered questions + sessionId
                                          409 MAINTENANCE_SCHEDULED {startsAt, endsAt} for a start
                                          that would run into a window
                                          startOrResumeAttempt accepts resume; a Continue never
                                          starts an attempt (#237)
GET    /app/attempts/:id                → Attempt + questions + serverTime + audioPlays
                                       a question's audio and a group recording's policy are
                                       StudentAudioPolicy: maxPlays, allowSeek, showTranscriptAfterSubmit
                                       and allowPause, all present
                                       optional session names the tab's held session;
                                       a different session on a live attempt answers
                                       200 with superseded:true, that held session id,
                                       and an empty beacon token, without a write
PATCH  /app/attempts/:id/answers        {sessionId,answers:[...],events:[...]} → {savedAt, serverTime, deadlineAt}
POST   /app/attempts/:id/events         standalone flush (sendBeacon path)
POST   /app/attempts/:id/audio-play     {questionId} → {plays, maxPlays}
POST   /app/attempts/:id/submit         idempotent; 409 if already closed; 409 DEADLINE_NOT_REACHED
                                          {deadlineAt} for timer_expired more than 5 s early
GET    /app/attempts/:id/result         → Attempt + review policy + sections + questions,
                                          each question naming its sectionId
GET    /app/media/:assetId/url          → short-lived signed URL
```

**Prefixes.** `/teacher/*` holds the teaching operations, `/admin/*` the platform's, `/app/*` the student's and `/auth/*` the caller's own session and account; `/me/*` accepts only the `self` requirement. Every operation that requires a bearer token declares its permission as `x-permission` (§5). Under `/teacher/*` and `/app/*`, an id the caller does not reach answers exactly as a missing one does. The v0.7.0 paths under `/admin/*` that moved still answer until v0.9.1 (T-R3.3): the server rewrites each to its new path (`DELETE /admin/students/:id` becomes `DELETE /admin/users/:id`) and logs `legacy_admin_path`.

**`User`** adds only optional `displayName` and `avatarUrl`; it never carries
phone, locale, timeZone or preferences. Student class/intro/join-preview teacher
names use the chosen display name, falling back to full name, without a new
private field. `CurrentUser.avatarUrl` is the caller's photo as a presigned GET valid 24 hours, set by `PUT /me/avatar` and cleared by `DELETE /me/avatar` (T-R4.8); `User` leaves it unset until a list that shows other people's photos has its own task.

**`CurrentUser`** adds optional private `phone`, `locale`, `timeZone` and
`preferences` to `User`'s fields, plus `permissions`, the keys the user's role holds in catalogue order, and `workspaces`, the consoles the user may open (`teacher`, `admin`, `app`). Only a response about the caller carries it: login, Google sign-in, `GET` and `PATCH /auth/me`, and `POST /auth/google/link`. A payload about someone else, such as the student on an attempt under review, carries `User`, so one user's permissions never reach another user's payload.

`deleteClass` and `deleteUser` answer `RESOURCE_REFERENCED` with `details.referencedBy`, which names what still references the row: `assignments`, `attempts`, `audit`, `members`, `owned_content` or `other`. Later releases add values, and a client treats one it does not know as `other`. The other permanent deletes (`deleteAssignment`, `deleteTest`, `deleteTestVersion`, `deleteQuestionGroup`) answer `RESOURCE_REFERENCED` without details.

**During a maintenance window**, every route answers `503 MAINTENANCE`, with `details {startsAt, endsAt}`, `Retry-After` (seconds until the end) and a vi/en message. The exceptions are `GET`/`HEAD` `/livez`, `/healthz` and `/public/status`. The 503 comes before authentication and rate limiting, so an expired token also gets it. A `POST /auth/logout` refused this way still clears the refresh and docs cookies when it carried the refresh cookie. The family is not revoked: the device is signed out, but a copy of the token held elsewhere stays usable for as long as it is rotated, because each rotation renews the lifetime and the cleared cookie can no longer trigger reuse detection; only a password reset, a disable or a password change ends it. The same holds for a logout the limiter refuses (429) and one whose revoke fails (500).

**A request the contract does not describe** is answered in the envelope, in the caller's language and with the request id. A path nothing serves answers `404 NOT_FOUND`, without the maintenance window being read. A path the router cleans (`//auth/me`, `/a/../b`) is first redirected to the cleaned path with 307, as before. A path under a method it does not serve answers `405 METHOD_NOT_ALLOWED` with an `Allow` header naming the methods it does serve; during a window it answers the 503 as every request to that path does. `HEAD` is answered as `GET` without a body on the operations that need no token (`GET /public/status`), and on `/livez` and `/healthz` as before; on a `GET` that needs a token it answers the same 405, with or without a token. Neither answer reaches a handler or spends a rate-limit bucket.

**Body readers and refusals.** The rate limiter's body key reads first on
login, Google sign-in, join preview and in-app join; the body-limit middleware
reads next, and the contract validator reads the buffered body last. A
non-streamed body the limit middleware cannot finish reading answers
`408 REQUEST_INCOMPLETE`: the request may be sent again. The earlier body-key
reader can swallow a read error on those four operations, so a declared body
that ends early can still answer 400 instead of 408. Streamed uploads have
separate handling. Every operation taking a JSON body declares the
validator's 400 response; malformed or schema-invalid bodies are
`400 VALIDATION_FAILED`, and so is a request that carries the character U+0000 in
a body string, a body member name, a query parameter or a path segment (v0.83).

**The AI assistant** (D4). With no provider (`ANTHROPIC_API_KEY` unset, so `Assembly.AI` is nil) every `/teacher/ai/*` and `/admin/ai/*` operation answers `501 NOT_IMPLEMENTED` with the code `AI_OFF`, before it reads anything, and `PublicStatus.ai` is false. Otherwise a suggestion operation, which needs `ai.use`, goes in this order: it reads what it will send through the caller's own scope (another teacher's id answers as a missing one), reserves its feature's ceiling in credits (generate 200, check 300, variant 300, explanation 20, wrong options 20, score 30, insights 60, bank clean-up 200), calls the provider, and ends the reservation with `Finish`, the one place that settles or releases. `Finish` settles what the provider billed, including the tokens of a refusal or of an answer that could not be used, and releases the rest, also when the handler panics or the caller goes away. The answer carries `usage {credits, left}`.

- **Refusals.** A paused teacher is refused before any lock: `403 AI_PAUSED`. An allowance or a pool that cannot cover the ceiling is `402 AI_CREDITS_EXHAUSTED` with `details.scope` `teacher` or `pool`, and nothing is held. `403 AI_READ_OFF` is a teacher whose `readStudentAnswers` is off asking for a feature that sends a student's answer. A provider that stays down after three retries, refuses, answers something no decoder can use, or rejects the request (a bad key or account, logged at ERROR for an operator) is `503 AI_UNAVAILABLE` with `Retry-After: 30`, and nothing is written. The other codes are `AI_REQUESTS_OFF`, `AI_REQUEST_PENDING`, `AI_REQUEST_DECIDED`, `AI_REQUEST_EXPIRED` and `AI_ALLOWANCE_BELOW_USE`.
- **The bank clean-up is one pass.** It reserves its 200 credits once, before the first call, and plans only the calls whose worst case at the configured prices fits that reservation. It makes at most five calls at the same time (up to four of 50 questions to tag, and one that judges the closest pairs of prompts) inside 45 seconds, which with the 10 seconds allowed to settle keeps it under the proxy's 60-second idle timeout, and ends with one `Finish` of the usage of every call the provider billed, on success, on a failed call, when the caller goes away and when a call panics. A call that fails, a call cut off by the time and a call the reservation could not cover leave what the other calls found in the answer with `truncated` true, and what was billed is settled; only a pass in which no call gave a usable answer is `503 AI_UNAVAILABLE`. A rewording that shares few words with the original is not found, and neither is a prompt typed without its diacritics next to its accented twin, because pairs are chosen by the words their prompts share, as typed; a duplicates call too long for its cap is cut to the best pairs that fit and the answer is `truncated`, never refused.
- **What a suggestion is checked against.** A score is moved to the nearest multiple of 0.5 up to the question's points; a wrong option that repeats an option is dropped; a clean-up tag for a question the call did not ask about, and a duplicate pair the call did not offer, are dropped; a formatted question is read for its words and never rewritten.
- **What leaves the system (D20).** Each prompt carries only what its feature needs. Generate: the pasted or chosen passage (its first 8,000 characters), the types, count, level and language. Check and variant: the draft's prompts, options, keys, explanations and passages, up to 120,000 characters; a check reads the members of the test's groups with their groups' passages, a variant only the standalone questions. Explanation and wrong options: the one question. Bank clean-up: the prompts, types and tags of the caller's own questions, newest first, at most 2,000 read, named q1 to qN in every call so that no id is sent; no call carries more than 50 questions to tag (the first 200 that have no tags, with the tags the bank already uses so a suggestion reuses them) or more than the questions of the 100 closest pairs of prompts, and a question without a pair is never sent for duplicates. Score: the question, its accepted or sample answers and the student's answer text. Insights: per question the correct rate and the eight most frequent wrong answer texts with their counts. No operation sends a student's name, email, id or class, an attempt id, or who wrote what; `operations_test.go` and `ai_flow_test.go` assert it with a fixture whose student is named in every other field. The two features that carry a student's words also need `teaching.grading`, and the score suggestion only for a short answer waiting for a mark by hand. No AI response reaches a student payload.
- **The subset rule.** The three Admin writes that name one teacher (`setAiAllowance`, which also pauses and resumes, and the two credit-request decisions) call `access.CanActOn`; a target outside the subset answers `403 FORBIDDEN`. `cmd/maintenance ai-credits` runs the same handlers as System and is not asked.
- **Limits per signed-in user.** The seven suggestions and the console writes 10 a minute and 100 an hour, the bank clean-up 3 an hour, the credit request 5 an hour, and the two reads (`getAiUsage`, `getAiCredits`) 60 a minute and 600 an hour.
- **Notifications.** Three kinds join the nine, all for the teacher's bell and none with a switch: `ai.allowance_warning` (once a month, when use crosses the warning share), `ai.credit_request` (to every holder of `ai.credits.manage`) and `ai.credit_request.decided`. Their params are `used`, `allowance`, `credits`, `teacherName` and `decision`.

**Notifications** (R4). `GET /me/notifications`, `POST /me/notifications/read`, `GET /me/summary` and the five switches are the caller's own and need only `self`. A notification is a `kind` and its `params`; the server sends no sentence and the reader's console words it. Twelve kinds exist: for a teacher `attempt.submitted`, `attempt.flagged`, `assignment.closing`, `class.joined`, `join_codes.rotated` and the three AI kinds above, and for a student `assignment.opened`, `assignment.due_soon`, `assignment.extended` and `result.ready`. Five switches govern seven of them: `attempt.submitted`, `attempt.flagged`, `assignment.closing` and `result.ready` each their own kind, and `assignment.due_soon` the student kinds `assignment.opened`, `assignment.due_soon` and `assignment.extended`. `class.joined`, `join_codes.rotated` and the three AI kinds have none. A switch that is off writes nothing. A list is newest first, 20 by default and 50 at most, and is the only keyset list in the contract (`before`); every other list is offset-paginated (O-20). `audience` (v0.79) narrows a list, a count and a marking to one console's kinds.

*Who is told, and when.* A notification is written after the change that caused it has committed, under a budget of its own that the end of the request does not cancel; a failure is logged and never fails the request. The teachers are the assignment's creator and the teachers of a target class the student belongs to (`visibility.PaperReaders`, the rule that lets a teacher see a paper, read the other way), each once, so a teacher of another class is told nothing about this student. `attempt.submitted` merges the papers handed in on one assignment into one notification per fifteen minutes, a timed-out paper being a hand-in; `attempt.flagged` is written once, when the integrity policy flags a paper, and never for a teacher's own flag or under `warn`; `class.joined`, which has no switch, on a join by code; `join_codes.rotated` by the start-up rotation (§6.1). The student is told `assignment.extended` when `notify` is true on an extension (everyone whose close it moved) or on an override that moves a close (those it gave time past the assignment's own close), and `result.ready` when a teacher finishes grading and the review policy shows the score at that moment; a `result.ready` notification carries the title and the attempt, never a score. *Due-time items.* `GET /me/summary` and `GET /me/notifications` first write what the passing of time has earned the caller, at most once in five minutes per caller in the serving process; there is no timer and no job. A moment older than seven days is not made up, a row the caller already holds is left as it is, read or not, and a switch that is off writes nothing. For a student: `assignment.opened` (when the assignment opened or was published, whichever is later), `assignment.due_soon` a day and an hour before the student's own close (an override and an early close included) while they have handed nothing in and have an attempt left and the test was available at that moment, and `result.ready` at the close of an `after_close` release for a paper with nothing waiting for a mark. For a teacher: `assignment.closing` an hour before the assignment's close, with the number of students they reach who have not handed in and whose own close is the assignment's. A close that moves earns its reminders again.

**The review options and the note.** `review.release` is `on_submit` or `after_close`; `review.showClassAverage` and `studentNote` are the teacher's other two choices. On `createAssignment` an omitted field takes its default (`on_submit`, false, no note); on `updateAssignment` an omitted one keeps what is stored, every other field is replaced, and `studentNote: null` clears the note. The note is trimmed by the server and stored as NULL when nothing is left; the contract counts its 500 characters before trimming. It is plain text everywhere it is drawn and is carried to a student on the assignment's detail only.

**The live lock.** While the derived status is `open` (from `opensAt` up to, not including, `closesAt`, and before an early close), `updateAssignment` refuses a change to `testVersionId`, `durationMinutes` or `maxAttempts` with `409 ASSIGNMENT_LOCKED`, whether or not anyone has started; sending the stored value is not a change. The check precedes the version check, so a scheduled or closed assignment that has attempts still answers `VERSION_LOCKED` for a changed version, and its timing stays editable.

**The closed rule.** An assignment that is closed (its window has ended, or it was closed early) stays closed under `updateAssignment`. A `window` that would make it scheduled or open again, or that opens after the early close it keeps, answers `409 ASSIGNMENT_CLOSED`: that is a reopening, and `reopenAssignment` does it with a reason and an audit row. The two operations take complementary sets, as `extendAssignment` and `reopenAssignment` do. A save that leaves it closed is unaffected: the stored window, an earlier close, `closeNow` (while the window has opened: with `opensAt` ahead of now it is `400 VALIDATION_FAILED` naming `closeNow`, for every status), any other field, and a later close on an assignment closed early (a save never lifts an early close).

**Release after the close.** The effective close of an assignment is the earlier of its early close and `closesAt`. Until `now` reaches it, a result whose assignment releases `after_close` is read as if the policy hid the score, the marks, the answer key and the explanations, and its grader's comments are not sent: the result's `review.showScore`, `showCorrectAnswers` and `showExplanations` are false, `review.release` and `review.showClassAverage` are as stored, and `releasesAt` is that close. A pending manual answer and a transcript keep their own gates. The student's assignment card shows no score before the same moment.

**`classAverage`** on a result is the mean, over the students who qualify, of each one's best graded attempt as earned over total, as a percent (0 to 100, rounded to two decimals). A student qualifies when they are targeted (through a class or by name) and enabled and have a graded attempt; voided, submitted-but-ungraded and timed-out-but-ungraded attempts count for nothing. It is present only when `review.showClassAverage` is on, this result is released, the assignment has closed and at least 3 students qualify, and no student is named.

**`liveAnsweredCount`** on a student's assignment card is the number of the live attempt's saved answers that say something, by the rule the engine's navigator applies (`web/src/features/take-test/answered.ts`): a choice with an option picked, a true/false with a value, a text that is not blank, and a fill-in with every blank of the frozen question filled. Blank means empty after removing the whitespace JavaScript's `trim()` removes. An answer that exists only in the browser's draft is not counted. The field is absent when there is no live attempt. The teacher's monitor applies the same rule to a row's `answeredCount`, so a saved answer the student has since cleared counts on neither screen.

**`classIds`** on a student's assignment card is every class the assignment targets that contains the student and is not archived, empty for a student targeted only by name, so a paper given to two of the student's classes can be next on both class cards; `classId` and `className` are set only when it holds exactly one class.

**The result's `sections`** are the paper's parts in test order, and every result question names its `sectionId`. Both are present under every review policy, because the attempt already showed the student its parts; the page sums a part's score from its questions' `earned`, which the policy still gates.

The teacher's lists return `{ items, page, pageSize, total }` with the list's own facets, and are offset-paginated by `page` and `limit` so a client can draw numbered pages (O-20 overrides §13.8's keyset rule for them); `GET /me/notifications` returns `{ items, nextBefore }` (keyset). Student payloads never include `isCorrect`, `sampleAnswer`, `acceptedAnswers`, or `transcript` (the last only per `showTranscriptAfterSubmit`, on the result endpoint).

---

## 16. Delivery phases

Each phase ends deployable. Do not start the next with the current one red.

| Phase | Deliverable | Exit criteria |
|---|---|---|
| **0 — Scaffold** | Vite app, Tailwind, shadcn (neutral), router, i18n, API client, lint/test/CI. **Neon skill installed**; `app` schema + users/identities/classes migrations. R2 bucket + credentials. | `pnpm dev/test/build` green; `/login` renders; migrations clean |
| **1 — Auth, join, shells** | Password login, Google login, refresh + rotation, **join-code flow end-to-end**, guards, both layouts, settings | E2E 2, 3, 4 pass |
| **2 — Admin authoring** | Tests list, builder (all 5 types), question bank, **media upload + validation**, publish + version snapshot | E2E 1 passes |
| **3 — Assign & take** | Assignment create/list, student home, intro, **take-test engine**, **audio player**, **integrity capture** | E2E 5, 6, 7, 8 pass |
| **4 — Grade & results** | Monitor, attempt review + integrity timeline, grading with sample answers, result page honoring review + transcript flags | Full E2E suite green, incl. 9 |
| **5 — Hardening** | Perf budgets, a11y pass, empty/error audit, mobile QA at 360px, rate-limit verification, `EXPLAIN` review of hot queries | Lighthouse a11y ≥ 95; budgets met; no seq scans on hot paths |

P1 after v1: presigned direct-to-R2 upload, CSV import (students, questions), password self-signup with email verification, dark mode, per-student time accommodations, per-question analytics.

The next accepted milestone is [production Word exam import](quizzivy-word-import-spec.md),
implemented through [the six-stage delivery plan](plan/17-word-import.md) and
[teacher workflow](plan/18-word-import-ux.md). The supplied document defines the
target rather than current availability. Existing free-form `.docx` and `.doc`
documents, explicit answer evidence, shared context, semantic formatting,
recoverable review and atomic draft creation are required before general release.
Imported and manually authored exams must share rendering, publication and
grading contracts. Missing answers remain unknown; OCR, answer generation and
automatic publication are excluded. PDF input is limited to a text layer (D-10) unless the teacher asks for "Read with AI" (below), and a test pasted as plain text is accepted as an exam source too (D-11). Each accepted detailed
contract updates the relevant sections above and OpenAPI before implementation.

**D4, the deck update and the AI assistant** (v0.11.0; `docs/plan/74d-d4-deck-update.md`). The release after R4 rebuilds the take-test engine, the builder's shared content and the three previews to the v2 deck, and adds the AI assistant (D19 to D22): the provider adapter and its tiers (T-D4.30), the credits (T-D4.31), the sixteen operations (T-D4.32), "Read with AI" in the import (T-D4.33), the teacher's surfaces and Settings › AI assistant (T-D4.34 to T-D4.39), and these documents (T-D4.40). It ships when, with the key unset, every AI operation answers 501 and no entry point is drawn; with the key set, a live smoke meters one generation, one score suggestion and one scanned page and reads them back from the ledger; and a reservation that would overspend the pool is refused by its constraint. The Admin's AI credits page is R5's (T-R5.35), on D4's operations.

The native source intake checkpoint is configured separately from learner media.
`IMPORT_S3_BUCKET` names a separate private bucket; `IMPORT_WORK_DIR` is an absolute,
private disk directory. Both are required to enable intake. The existing S3 endpoint
and credentials are reused, with no public bucket or CDN fallback. An import, its
sources, its review and its commit belong to its creator. `scope.all` reaches every
import by id, the history lists only the caller's own (§5), and another creator's
import answers as a missing one does. The test a
commit creates, with its questions and groups, belongs to the import's creator; the
committer stays their `created_by`, the audit actor and `committed_by` (T-R2.12f).
Creator and modifying actors are retained.

Availability is reported, not assumed. `GET /teacher/imports/capabilities` answers on
every deployment:

- `intakeEnabled` is true when the import store is configured.
- `processingEnabled` is true when `IMPORT_PROCESSING_ENABLED` states that a
  worker runs beside the API. The API refuses that setting without import storage.

Without processing, `processWordImport` answers 503 `IMPORT_PROCESSING_UNAVAILABLE`
and queues nothing, so no new run waits for a worker that does not exist. A run
queued before processing was switched off waits until a worker returns or the
teacher cancels it.
Finished imports stay reviewable and committable. The client hides the feature
where intake is off. Where processing is off, it withholds new imports, retries
and reprocessing, and says why. Production runs both: O-24 was decided on 2026-09-25, and the
import worker runs there as its own Fly process group.

The R4 history presents the existing list contract as one card/grid that changes
at 960px of its outer container. Six ordered filter controls, search, page and
page size use the URL; active-import polling pauses while idle and refreshes on
resume. History returns six facets under the caller's own scope and search,
ignoring the selected status. Repeated status values are OR-ed, with a single
value still valid; Processing selects `awaiting_sources`, `queued`, `processing`
and `committing`. Each history item carries `reviewCounts {needsAction,
toConfirm}|null`, derived from the exact current review body. Machine completion,
review save and candidate adoption update counts with that body; an unadopted
candidate leaves current counts intact. An untouched historical draft remains
unknown, while computed zero is an exact zero. Facets, pagination and hydrated
items share one repeatable-read snapshot. Text sources carry their normalized
character count; filename search excludes only their stored `pasted-text.txt` name,
while title and companion-file search remain available.

The R4 upload page keeps one source-intake instance mounted across
`?source=paste` changes, preserving selected files, the pasted text and the typed
title. File mode retains automatic recognition, actual limits and retention, the
privacy notice and a sticky Cancel/Start footer. Leaving during upload quietly
stops intake, as before. Neither page infers provider policy from capability data.
Paste mode is a plain textarea with a counter against `pasteMaxCharacters`
(code points of the text as it will be sent, composed to NFC), no answer-key slot,
and a "Found so far" count of sections, questions and answers. That count follows
the server's recognition rules for text sources, not a rule of its own, and
`api/testdata/pasted-text-counts.json` holds both to the same numbers; it is a
hint, and the review is the authority. Start processing is disabled for an empty
box, a text over the limit and a text with no numbered question, and runs the file
flow's steps with the text in place of the exam file: create, paste, process.
Pasted text that has not started asks before the teacher leaves.

The R4 processing page uses the deck's 720px frame and five presentation rows
mapped to the actual run stage. It retains elapsed time, attempts and retry waiting,
shows completed receipts only after their stages, and exposes no invented run-event
history. The actual exam source's role and format select the pasted-text header,
character receipt and plain-text/no-formatting receipt, independently of filename
or title. Failed text imports keep the actual error code and stage; source-invalid
and size titles and recovery instructions remain neutral about the pasted text or
its companion file. Restart opens the paste mode. Cancellation and closed states use configured retention and finality; Ready
uses the current review's real counts, severity and conflict rules. The active-stage
spinner pauses on hover and keyboard focus and stays static under reduced motion;
retry waiting has no active spinner. This page's source title is static.

`POST /teacher/imports/{id}/sources/text` accepts a closed JSON body with
`uploadId`, `expectedRevision` and plaintext `text`, under `content.tests.write`.
It normalizes NFC and stores a private `pasted-text.txt` exam source using the
file upload's ownership, replay, revision, quota, intake-capacity and retention
rules. `ImportLimits.pasteMaxCharacters` is 100000; file `formats` stay unchanged.
The request schema rejects more than 100000 sent code points with 400. The command
rejects NUL or invalid UTF-8 with 415, and more than 100000 normalized code points
or 20000 nonblank lines with 413. Invalid UTF-8 is a command boundary assertion;
JSON decoding can replace malformed wire bytes. The default raw-body limit still
applies. A multipart `.txt` file remains unsupported.

Text extraction removes an initial BOM, normalizes line breaks and NFC, and emits
plain paragraph blocks with original line-number IDs, no formatting spans and
`TEXT_MARKS_UNAVAILABLE`. The private original and existing download, source view
and retention operations remain available. `word-pipeline-v3` fences these runs
from v2 workers, and a run still queued under v2 when v3 is deployed fails after ten
minutes with `PIPELINE_RETIRED`, so its teacher processes it again. A companion
answer-key file uses the existing upload operation. A text source is never normalised
or converted, so a worker needs no converter to read one (`docs/setup/word-import-worker.md`).

**A pasted text is plain text.** Whatever the clipboard held, only its text is stored:
the page reads no HTML, so no bold, underline or colour exists to read, as with a PDF.
Every pasted draft says so in an informational finding, `TEXT_MARKS_UNAVAILABLE`.
Images and audio are not pasted; the page says so and the test builder adds them. A
question that depends on an underline is marked by the teacher in review. The source
view shows the one source as pasted, "Download original" returns the stored `.txt`, and
the history names the source "Pasted text" with its character count, never by the stored
file name.

**Recognition is `rules-v3`.** For an exam whose evidence version is `text-lines-v1` the
recognizer also reads the forms the paste page tells the teacher to type, and no others:
`*` as the first or last non-space character of an option marks it correct (several
starred options of one question are one answer, and a `*` anywhere else is content); an
answer line under a question, `Answer: B`, `Ans: B`, `Answer - B`, `Key: B` or
`Đáp án: B`; lower-case option letters; a bare `1:` as a question label; a bare roman
numeral on its own line, and a named part such as `Part A` or `Bài tập 1`, as headings
and never as the title; and a run of three or more underscores as a gap. `Answer: Not
given` gives a short answer the teacher fixes in review, because true / false / not given
is not a question type before R6. **A one-line answer key is read for every format, Word
and PDF files too** (#233): `Answer key: 1-B 2-C`, also `Answers:`, `Key:` and `Đáp án:`
before the pairs, whose separator is `-`, `:`, `.` or `)`, is the key's entries when it
holds at least two pairs, so `Đáp án: 1-B 2-C` is no longer attached to the last question
as one value. A line with fewer than two pairs keeps its old meaning. Formatting is never
evidence of correctness; these conventions are text. `docs/plan/17-word-import.md` §1.41
records the scope and what is measured.

`/teacher/imports` creates an empty record idempotently and lists history by status,
title or current filename. A source upload accepts exactly one native `.docx`, with
bounded package/content inspection, a maximum 25 MiB compressed body and the Word
inspector's expansion/XML limits. `.doc` is accepted where `IMPORT_LEGACY_DOC` is
set. Operators set it only beside a worker that has the isolated converter; a run
without one fails with `LEGACY_CONVERSION_UNAVAILABLE`. A successful upload only
acknowledges stored source bytes.

A `.pdf` is accepted by its `%PDF-` signature. The worker reads it with PDFium
inside a WebAssembly sandbox: no filesystem, no network, 256 MiB of memory, one
minute, 60 pages. It never converts a PDF. It reads only the text layer, one
line per block. Page numbers are kept out of the exam. Lines repeated at the top
or bottom of the pages are kept out too, and the teacher confirms them, since a
repeated instruction may belong to the exam. A line holding two questions side
by side, as a two-column page produces, needs review. A PDF carries no underline,
bold or colour marks for recognition, and the draft says so in an informational
finding. A scan, a locked
file, a broken file or one over the limits fails with `PDF_NO_TEXT`,
`PDF_PROTECTED`, `PDF_INVALID` or `PDF_TOO_LARGE`, and the teacher needs another
file; a scan is read instead when the import asked for "Read with AI".

**Read with AI** (D4, T-D4.33; `docs/plan/17-word-import.md` §1.42). `createWordImport` takes `readWithAi` (default false), stored on the import in `word_imports.read_with_ai` and fixed from then on. It is refused with `422 VALIDATION_FAILED` naming `readWithAi` when the server has no assistant or the caller lacks `ai.use`. It matters only to a PDF that fails the text-layer check: a `.docx`, a pasted text and a PDF with a text layer take the path they always took, send nothing and spend nothing. The worker draws up to 40 pages in the PDFium sandbox (a longer file fails `PDF_TOO_LONG_FOR_AI` before anything is held or sent; a scan over the general 60-page limit stays `PDF_TOO_LARGE`), reserves 60 credits a page for the whole file before the first call (a shortfall is `AI_CREDITS_EXHAUSTED` or `AI_PAUSED` with nothing sent), and sends the pages to the capable tier four at a time. The teacher who started the run pays, in one ledger row for the file. What comes back is each page's printed lines, not questions: they become `pdf-lines-v1` evidence, and the recognizer, its source links, review and the transactional commit are the text layer's, so the provider is an untrusted reader and writes no field of a draft. A page the service cannot read is a review-required finding on that page, never a silent gap; every page read counts toward one informational finding, "{n} pages were read by an AI service outside Quizzivy", which is the import's record of what left the system; a file with no readable page fails `AI_UNREADABLE`, and a provider that stays down fails `PROVIDER_UNAVAILABLE`, which is terminal because a retry would pay for the pages again. An import whose scan was read keeps the extraction, so processing it again spends nothing. The Sources pane says that scanned pages are read by an AI service outside Quizzivy and that pages with selectable text are not sent. No check refuses a file that names a student: it would be a false sense of safety, and the teacher carries the responsibility for the file.

Each accepted exam/key addition or replacement creates an immutable source set,
retaining its unchanged companion and prior originals. Uploads require the current
import revision while `awaiting_sources`. Exact retries reuse their upload identity;
changed input conflicts. A durable reservation precedes each object write, and both
pending and completed bytes count towards actor/global quotas. No database transaction
is held during upload or inspection. The API admits one expanded inspection per
process. Interrupted reservations remain observable until retention removes the
import's files. Only completed sources can receive a 60-second download
URL, forced to attachment/octet-stream. Source identifiers are never media asset IDs.

Retention keeps an import's original files, artifacts and review draft as follows:

- **After commit:** 30 days, counted from the commit.
- **After cancellation:** 7 days.
- **Idle imports:** an import waiting on a teacher (`awaiting_sources`, `failed`,
  `needs_review`) that nobody has touched for 60 days, draft saves included, is
  closed as `cancelled`, flagged `closedIdle`, and its files are removed at once.
  An upload, a reservation or a draft save counts as a touch.

The API sweeps at start-up and then daily, and retries a failed sweep after an
hour. It exists wherever import storage does, with or without a worker. Each
sweep works through everything due, a batch at a time, within a five-minute
budget. An import whose removal fails is passed over for the rest of that sweep.

For each import the sweep deletes the objects first, then the draft, then sets
`files_removed_at`, and it audits the closure and the removal. Removal applies
only to committed or cancelled imports, which can never be processed again.

The history row, the source metadata, the runs and the commit record remain.
Downloads, the source view and the review answer 410 `IMPORT_FILES_REMOVED`, and
the client says the files were removed under the policy.

Native extraction retains XML order and source-bound identities for paragraph,
container, object and unassigned-content blocks. Source fragment offsets use
Unicode code points and exclude generated labels. Hidden/revised text, fields,
ancillary parts and unsupported objects remain review evidence. No formatting is
interpreted as correctness without a confirmed convention. Nested table coordinates
retain merge evidence; ambiguous grids are flagged. Selected PNG/JPEG assets may
be normalized under bounded decoding limits, but remain private until explicitly
reviewed and bound to learner content. Source blocks are separate from machine
candidate JSON and must not be exposed on student endpoints.

An internal offline converter can normalize binary DOC and render private source
pages under fixed development resource limits. Native originals remain unchanged;
converted DOCX and raster output are revalidated. Source, renderer, immutable image
and artifact identities accompany the rendition. Layout and legacy conversion
require explicit review; visual source pages do not imply a question-coordinate map.
A single Docker slot prevents orphan/retry overlap, and the container's independent
deadline remains active after worker failure. This tool is not yet public legacy
intake or production activation.

Private stage artifacts have a durable reservation before object storage, pinned
by source revision, role, run, claim and component configuration. Pending bytes
count towards separate actor/global limits. A set becomes readable only when all
its declared files are stored; completed files and sets are immutable. Current
claims may reuse completed evidence for the same source, pipeline and component,
including after an explicit retry. Incomplete evidence from an older claim is
retained for accounting but cannot be adopted by a newer worker. Conditional,
checksum-verified writes prevent changed replay from replacing stored bytes.
Source blocks and page files stay outside the bounded run-result JSON. There is
no learner access through these objects; they are removed with the import's other
files under the retention policy.

Processing requests retain their source-set revision, pipeline version and replay
identity. One queued/running request per import is permitted, with at most 50
retained requests per import and 1–10 automatic attempts per request. Claims and
heartbeats serialize capacity accounting across worker processes. Live leases
expire after a configured 1 second–5 minutes; every state/result write checks the
worker identity, fencing token, lease and source revision. Exhausted crashes become
failed runs. Retrying a failed import creates a new immutable run identity.

The offline deterministic recognizer produces `word-candidate-v1` proposals from
source-linked text, never bank questions. It distinguishes unknown, known and
conflicting choice keys; explicit option IDs survive reordering. Numbering,
sections/papers, same-line options, continuation paragraphs and explicit inline,
final or companion choice keys retain Unicode source ranges. Restarted labels and
ambiguous table associations do not authorize a guessed match. Bold/underline only
become answer evidence under an explicitly teacher-confirmed convention; key-only
marks are removed from the proposed learner prose.

Every meaningful block remains in the coverage ledger. Unassigned ranges, private
branches, uncertain structure, default points and unresolved fidelity are findings.
Source comments, hidden/revised text and fields are never automatically copied into
learner prose. This initial recognizer handles labeled choice structures; grouped
cloze, typed/written keys, saved profiles and assisted free-form recognition remain
open. A proposal is not a reviewed draft or approval to commit an assessment.

An internal runner stops cooperating processors on timeout, lease loss or shutdown.
Processing runs outside transactions. Only a current claim can store a bounded
private JSON object and transition to `needs_review`. Terminal runs cannot be
changed. Run events retain actor/worker/stage/failure codes without document contents.
The standalone `import-worker` assembles source verification, private rendition,
chunked raw extraction and deterministic recognition. It verifies object lengths
and SHA-256 before parsing, reuses completed stages pinned to source/configuration,
and stores candidate content privately. Its bounded result contains only lineage
and artifact-set identities. The API does not start this worker or require Docker.
One synchronous job per process, database lease limits, a 512 MiB Go soft-memory
target, a five-minute job deadline and the converter's separate hard limits are
conservative development defaults, not the approved production capacity envelope.
Full domain validation, public processing controls and production supervision
remain required before release.

Thuong approved two ownership/recovery policies for this milestone:

- Groups and shared materials belong to independent editable copies. Saving a
  dependent group to the bank or copying it into another test carries every
  required member/material and remaps identities; subsequent source edits or
  deletion cannot affect the copy. Relational asset references protect reused
  immutable files from deletion; reuse of bytes is not shared editable content.
- Unsent edits may be stored locally, isolated by account, for at most seven days
  from the unsent revision's creation, and cleared on logout. Recovery is explicit
  and revision-aware; local-only changes never display as server-saved. Local
  storage failure or quota exhaustion must remain visible and cannot silently
  discard unsent changes. Source/answer files are not stored in this recovery
  outbox. The group editor implements this through IndexedDB, with expiry checked
  before recovery, explicit restore/discard and a global logout fence that rejects
  older writers in other tabs. Opening a newer editor of the same item fences the
  previous local writer; it cannot overwrite or clear the newer editor's outbox.
  Section-owned group editing uses the same recovery gate and outbox. An explicit
  route exit first confirms local persistence; save-and-leave flushes both content
  and outline. Test title/outline and standalone questions are not stored locally.
  Import review and standalone authoring recovery still require integration.

---

## 17. Decisions taken inside the answers — worth a second look

All original open questions are closed. These sub-decisions were made while implementing the answers, and are the ones most likely to want changing.

1. **Self-join is Google-only** (§6.3). Password self-signup needs verified email, which needs an email provider. If Thuong wants that in v1, add Resend/SES and an `email_verifications` table — the join flow itself is unchanged. *Decide before Phase 1.*
2. **No admin approval queue for self-join** (§6.5). Rotate-and-remove is judged sufficient for a small practice. If enrolments become high-volume or a code leaks in practice, add a `pending` membership state. *Non-blocking.*
3. **mp3 and m4a only** (§11.1). Covers every current browser with no transcoding. Adding `.ogg`/`.wav` means a transcoding pipeline or a Safari support matrix. *Decide before Phase 2.*
4. **Cloud AI processing is on for the assistant's features** (D19), **with per-feature minimisation** (D20) **and a pool-constrained budget** (D21), decided by Thuong on 2026-10-10. The provider is Anthropic's API. Each feature sends only what it needs and none sends a student's name, email or id (§15). The centre's $200 a month is a hard stop that only credits an Admin adds can lift, and the product bills nobody (§13.10). Removing `ANTHROPIC_API_KEY` turns the assistant off and changes nothing else. *Two pieces stay open and block nothing: the student-facing sentence about cloud processing on the Test intro waits for Thuong's wording and is not built, and the provider account's data-retention setting is his to confirm before the key is set in production (`docs/plan/70-redesign-overview.md` §10).*

---

## 18. Working agreement for the AI agent

- Read this spec fully before starting any phase. Where §5, §6, §7, §10, §11, §12, or §13 is ambiguous, ask **one** precise question rather than guessing.
- Install and consult the Neon `postgres-best-practices` skill before writing any DDL or non-trivial SQL. Cite the PG18 docs section for any version-specific construct.
- Propose before adding a dependency or deviating from §2, §12, or §13.
- Any endpoint reachable without a session gets a rate limit and a leak review in the same PR.
- Small, reviewable PRs scoped to one feature folder.
- Write the `vi` string first, then `en`. Never leave English-only user-facing text.
- When touching `features/take-test/`, `features/integrity/`, or `features/media/`, run their unit tests before and after every change.
- Never remove a failing test to make CI green; fix it or flag it.
- Keep this document current: when a decision changes, edit the section and record it in the changelog at the top.
- The version is numbered at release, not per PR. On an integration branch (`develop`, `work/**`) a task PR never edits the version line: it adds its bullets under the one heading `**Changes pending (R5, v0.12.0)**`, each starting with its task id (`T-R5.15 §15 …`). The release PR sets the version line to the next free number and renames that heading `**Changes since v0.NNN**`, where NNN is the version the release replaces. A hotfix off `main` numbers itself, and its back-merge carries the number down.
