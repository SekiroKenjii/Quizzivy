# Student layouts and interaction rules

This plan implements the student design contract in spec v0.7, §9 and §12.

## Layout and navigation

Home and classes use the full available width with adaptive grids. Assignment
intro and results use a centred 720px reading column. These pages no
longer reserve a sparse right panel. The exam retains its navigator at 256px by
default with a 224–384px range and a storage key separate from teacher panels.
The student outlet and settings forms keep their identity across 1024px.

Every active attempt is visible, ordered by deadline. Available assignments say
“View details”, because only the intro starts the clock. Class cards link to
home filtered by class. Home's class/status filters are encoded in the URL and
empty filters provide a reset. Completed cards retain class and submission date.

## Controls and feedback

Phone buttons, question cells, select items and dialog actions have 44px targets.
On phones, assignment cards put the class name below the deadline badge.
The 320px exam footer keeps previous and question-list icons beside a flexible
next/review button. Review submission stays outside the scrolling question list.
The success screen links directly to the submitted paper.

Single/multiple-choice instructions explain selection without revealing correct
answer counts. Restored answers say they were loaded. Listening continues after
the teacher's allowance under the existing record-only policy; the copy explains
that extra plays are recorded. Wrong-result filters are absent when score review
is disabled, and empty filters explain their state. The full title and summary
appear above result questions on both phone and desktop.

Settings shares the teacher layout: Profile, Security and Preferences URLs, a
192px desktop navigation column and a mobile section select. Forms remain mounted
across section changes. Divided rows keep labels separate from controls; the form
column is capped at 768px. Home status filters show counts beside their labels.
Cards use subtle shadows and pointer hover feedback; section transitions last
150ms and respect reduced motion. Password
fields support reversible visibility. Login links to class joining; signed-in
join pages identify the current account and offer return navigation.

## Acceptance

Regression tests cover multiple active attempts, class navigation/filter reset,
policy-aware result filtering, restored-save copy, selection instructions,
independent navigator width, 320px footer bounds, and state persistence across
1024px. Browser checks use 320, 360, 1024, 1440 and 1920px and both languages.
Existing grading, audio, auth, integrity and autosave tests remain in force.
