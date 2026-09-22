import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { BulkActions } from "@/components/shared/BulkActions";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { ApiError } from "@/lib/api/errors";
import "@/lib/i18n";

const items = [
  { id: "a", name: "First" },
  { id: "b", name: "Referenced" },
];

it("confirms the selection, retains failed items and retries only those items", async () => {
  const run = vi
    .fn<(item: (typeof items)[number]) => Promise<void>>()
    .mockResolvedValueOnce(undefined)
    .mockRejectedValueOnce(
      new ApiError({
        status: 409,
        code: "RESOURCE_REFERENCED",
        message: "Still assigned",
      }),
    )
    .mockResolvedValueOnce(undefined);
  function Harness() {
    const selection = useBulkSelection<(typeof items)[number]>();
    return (
      <>
        <button onClick={() => selection.selectPage(items, true)}>Select page</button>
        <BulkActions
          selected={[...selection.selected.values()]}
          name={(item) => item.name}
          actions={[{ label: "Delete selected", description: "Cannot undo", run }]}
          onRemoved={selection.remove}
          onClear={selection.clear}
          onSettled={() => Promise.resolve()}
        />
      </>
    );
  }
  render(<Harness />);
  const user = userEvent.setup();
  await user.click(screen.getByText("Select page"));
  await user.click(screen.getByText("Delete selected"));
  expect(run).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Xác nhận 2 mục" }));
  await screen.findByText("Referenced: Still assigned");
  expect(screen.getByText("Đã chọn 1 mục")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Thử lại 1 mục chưa thành công" }),
  );
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(run.mock.calls.map(([item]) => item.id)).toEqual(["a", "b", "b"]);
});
