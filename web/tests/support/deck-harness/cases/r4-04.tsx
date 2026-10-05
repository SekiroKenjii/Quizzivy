import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import { Archive, ClipboardList, Plus, UserPlus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PageHead } from "@/layouts/shell/PageHead";

const LONG_TITLE =
  "Kiểm tra giữa kỳ Tiếng Anh giao tiếp nâng cao cho lớp buổi tối thứ Ba và thứ Năm, khoá thu 2026";

export const cases: Record<string, () => ReactElement> = {
  "page-head-list": () => (
    <MemoryRouter>
      <PageHead
        title="Assignments"
        description="Tests you have sent to classes and students."
        actions={
          <Button>
            <Plus aria-hidden="true" />
            New assignment
          </Button>
        }
      />
    </MemoryRouter>
  ),

  "page-head-detail": () => (
    <MemoryRouter>
      <PageHead
        title="IELTS 6.5 Evening"
        back={{ to: "/teacher/classes", label: "Classes" }}
        status={
          <Badge variant="secondary" className="in-data-[scale=deck]:px-2.25">
            24 students
          </Badge>
        }
        actions={
          <>
            <Button variant="outline">
              <UserPlus aria-hidden="true" />
              Add student
            </Button>
            <Button>
              <ClipboardList aria-hidden="true" />
              Assign to class
            </Button>
          </>
        }
      />
    </MemoryRouter>
  ),

  "page-head-long": () => (
    <MemoryRouter>
      <PageHead
        title={LONG_TITLE}
        back={{ to: "/teacher/classes", label: "Lớp học" }}
        status={
          <Badge variant="secondary" className="in-data-[scale=deck]:px-2.25">
            24 học viên
          </Badge>
        }
        actions={
          <>
            <Button variant="outline">
              <Archive aria-hidden="true" />
              Lưu trữ lớp
            </Button>
            <Button variant="outline">
              <UserPlus aria-hidden="true" />
              Thêm học viên
            </Button>
            <Button>
              <ClipboardList aria-hidden="true" />
              Giao bài cho lớp
            </Button>
          </>
        }
      />
    </MemoryRouter>
  ),
};
