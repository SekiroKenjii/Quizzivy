/**
 * The panel drawings for the three failure screens (E-01..E-03), transcribed
 * from the deck rather than redrawn.
 *
 * They share a grammar so the three read as one family: a dotted path and the
 * thing at the end of it, geometry only, one stroke weight, `currentColor` at
 * two opacities. §12 rules out alarm iconography and these screens are where
 * that matters most — a student who hits a 403 has done nothing wrong, and a
 * broken page is the product's fault, not theirs. Nothing here is red, and
 * nothing carries an exclamation mark.
 *
 * They inherit colour from the panel, which is why they carry no fill of their
 * own. Each is `aria-hidden` at the call site: the words beside them say the
 * same thing, and a screen reader announcing "dotted path to a missing page"
 * would be a worse version of "Không tìm thấy trang".
 */

const SIZE = { viewBox: "0 0 260 260", width: 260, height: 260 } as const;

/** Pages that exist, and the route running past the last one to a page that does not. */
export function NotFoundArt() {
  return (
    <svg {...SIZE} fill="none" stroke="currentColor" aria-hidden="true">
      <g strokeOpacity="0.4" strokeWidth="2">
        <rect x="18" y="86" width="62" height="82" rx="8" />
        <rect x="99" y="86" width="62" height="82" rx="8" />
      </g>
      <g fill="currentColor" fillOpacity="0.28" stroke="none">
        <rect x="30" y="104" width="38" height="5" rx="2.5" />
        <rect x="30" y="117" width="28" height="5" rx="2.5" />
        <rect x="30" y="130" width="34" height="5" rx="2.5" />
        <rect x="111" y="104" width="38" height="5" rx="2.5" />
        <rect x="111" y="117" width="24" height="5" rx="2.5" />
        <rect x="111" y="130" width="32" height="5" rx="2.5" />
      </g>
      {/* The destination, dashed: the link was real, the page is not. */}
      <rect
        x="180"
        y="86"
        width="62"
        height="82"
        rx="8"
        strokeWidth="2"
        strokeDasharray="7 7"
        strokeOpacity="0.8"
      />
      <path
        d="M49 200 H130"
        strokeWidth="2"
        strokeOpacity="0.4"
        strokeLinecap="round"
      />
      <path
        d="M130 200 H203"
        strokeWidth="2"
        strokeOpacity="0.8"
        strokeDasharray="2 8"
        strokeLinecap="round"
      />
      <g fill="currentColor" stroke="none" fillOpacity="0.4">
        <circle cx="49" cy="200" r="4" />
        <circle cx="130" cy="200" r="4" />
      </g>
      <circle cx="211" cy="200" r="5.5" strokeWidth="2" strokeOpacity="0.8" />
    </svg>
  );
}

/** Ordered rows, one of which has come apart and slipped out of line. */
export function UnexpectedErrorArt() {
  return (
    <svg {...SIZE} aria-hidden="true">
      <g fill="currentColor" fillOpacity="0.3">
        <rect x="40" y="52" width="180" height="12" rx="6" />
        <rect x="40" y="76" width="150" height="12" rx="6" />
        <rect x="40" y="100" width="172" height="12" rx="6" />
      </g>
      {/* The break is the one element at full opacity, and it is still not red. */}
      <g fill="currentColor">
        <rect x="40" y="124" width="74" height="12" rx="6" />
        <rect x="136" y="132" width="62" height="12" rx="6" />
        <circle cx="125" cy="135" r="3.5" />
      </g>
      <g fill="currentColor" fillOpacity="0.3">
        <rect x="40" y="162" width="164" height="12" rx="6" />
        <rect x="40" y="186" width="132" height="12" rx="6" />
        <rect x="40" y="210" width="176" height="12" rx="6" />
      </g>
    </svg>
  );
}

/** The same dotted path, arriving at a boundary that is simply closed. */
export function ForbiddenArt() {
  return (
    <svg {...SIZE} fill="none" stroke="currentColor" aria-hidden="true">
      <rect
        x="128"
        y="60"
        width="104"
        height="140"
        rx="14"
        strokeWidth="2"
        strokeOpacity="0.4"
      />
      <g fill="currentColor" fillOpacity="0.22" stroke="none">
        <rect x="148" y="86" width="60" height="6" rx="3" />
        <rect x="148" y="100" width="44" height="6" rx="3" />
        <rect x="148" y="114" width="52" height="6" rx="3" />
      </g>
      <path d="M128 60 V200" strokeWidth="5" strokeLinecap="round" />
      <circle cx="44" cy="130" r="9" strokeWidth="2" strokeOpacity="0.55" />
      <path
        d="M60 130 H108"
        strokeWidth="2"
        strokeOpacity="0.55"
        strokeDasharray="2 8"
        strokeLinecap="round"
      />
      <rect
        x="114"
        y="122"
        width="28"
        height="22"
        rx="5"
        fill="currentColor"
        stroke="none"
      />
      <path d="M120.5 122 v-6 a7.5 7.5 0 0 1 15 0 v6" strokeWidth="2.6" />
    </svg>
  );
}
