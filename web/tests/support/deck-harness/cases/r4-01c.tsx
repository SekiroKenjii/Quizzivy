import { useState } from "react";
import { useTranslation } from "react-i18next";
import { FacetFilter, type FacetOption } from "@/components/shared/data/FacetFilter";
import { CardGrid } from "@/components/shared/CardGrid";

const CLASSES: readonly FacetOption[] = [
  { value: "evening", label: "IELTS 6.5 Evening" },
  { value: "foundation", label: "IELTS Foundation A" },
  { value: "weekend", label: "TOEIC 600 Weekend" },
  { value: "kids", label: "Kids Starters B" },
];
const LONG: readonly FacetOption[] = [
  {
    value: "long",
    label:
      "Lớp luyện thi tiếng Anh nâng cao vào buổi tối dành cho học viên cần củng cố toàn bộ kỹ năng đọc, nghe và viết",
  },
  ...CLASSES,
];
const COPY = {
  vi: {
    class: "Lớp",
    classTitle: "Lọc theo lớp",
    status: "Trạng thái",
    statusTitle: "Lọc theo tài khoản",
    cards: "Thẻ mẫu",
    states: ["Đang hoạt động", "Chưa đăng nhập", "Đã đặt lại mật khẩu"],
  },
  en: {
    class: "Class",
    classTitle: "Filter by class",
    status: "Status",
    statusTitle: "Filter by account",
    cards: "Sample cards",
    states: ["Active", "Not signed in", "Password reset"],
  },
};
const SIX = Array.from({ length: 6 }, (_, index) => ({ id: String(index + 1) }));
const EIGHT = Array.from({ length: 8 }, (_, index) => ({ id: String(index + 1) }));

function useCopy() {
  const { i18n } = useTranslation();
  return COPY[i18n.language.startsWith("vi") ? "vi" : "en"];
}

function useClassFilter(active = false, options = CLASSES) {
  const copy = useCopy();
  const [selected, onChange] = useState<readonly string[]>(
    active ? ["evening", "foundation"] : [],
  );
  return { label: copy.class, title: copy.classTitle, options, selected, onChange };
}

function grid(label: string, min: number, gap: 10 | 12 = 12, eight = false) {
  return (
    <CardGrid
      label={label}
      items={eight ? EIGHT : SIX}
      itemKey={(item) => item.id}
      min={min}
      gap={gap}
    >
      {(item) => (
        <div className="bg-card shadow-card grid h-30 place-items-center rounded-xl border">
          {item.id}
        </div>
      )}
    </CardGrid>
  );
}

/** cases supplies localized controlled filters and static grid fixtures for independent browser comparison. */
export const cases = {
  "facet-class": function ClassFilter() {
    return (
      <div className="flex justify-end">
        <FacetFilter {...useClassFilter()} />
      </div>
    );
  },
  "facet-class-active": function ActiveClassFilter() {
    return (
      <div className="flex justify-end">
        <FacetFilter {...useClassFilter(true)} />
      </div>
    );
  },
  "facet-status": function StatusFilter() {
    const copy = useCopy();
    const [selected, onChange] = useState<readonly string[]>([]);
    const values = ["active", "not-signed-in", "password-reset"];
    const options = values.map((value, index) => ({
      value,
      label: copy.states[index]!,
    }));
    return (
      <div className="flex justify-start">
        <FacetFilter
          label={copy.status}
          title={copy.statusTitle}
          options={options}
          selected={selected}
          onChange={onChange}
          clearable={false}
          menuClassName="data-[scale=deck]:w-55"
        />
      </div>
    );
  },
  "facet-empty": function EmptyFilter() {
    return (
      <div className="flex justify-end">
        <FacetFilter {...useClassFilter(false, [])} />
      </div>
    );
  },
  "facet-disabled": function DisabledFilter() {
    return (
      <div className="flex justify-end">
        <FacetFilter {...useClassFilter()} disabled />
      </div>
    );
  },
  "facet-long-vi": function LongFilter() {
    return (
      <div className="flex justify-end">
        <FacetFilter {...useClassFilter(false, LONG)} />
      </div>
    );
  },
  "card-grid-290": function TestsGrid() {
    return grid(useCopy().cards, 290);
  },
  "card-grid-300": function ClassesGrid() {
    return grid(useCopy().cards, 300);
  },
  "card-grid-240": function MediaGrid() {
    return grid(useCopy().cards, 240);
  },
  "card-grid-160": function MediaPickerGrid() {
    return (
      <div className="w-140 max-w-full">{grid(useCopy().cards, 160, 10, true)}</div>
    );
  },
};
