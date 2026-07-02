import { useEffect } from "react";
import { WorkbenchEventStream } from "../events/eventStream";
import { BoardFlow } from "../flows/BoardFlow";
import { TaskBottomPanel, TaskRightSidebar, TaskWorkbenchMain } from "../flows/TaskWorkbenchFlow";
import { useWorkbenchQuery } from "../queries/useWorkbenchQuery";
import { AppShell } from "../shared/ui/AppShell";
import { EmptyState } from "../shared/ui/EmptyState";
import { SplitPane } from "../shared/ui/SplitPane";
import { useWorkbenchStore } from "../state/workbenchStore";

const eventStream = new WorkbenchEventStream("ws://127.0.0.1:1421/events");

export function WorkbenchRoute() {
  const project = useWorkbenchStore((state) => state.project);
  const hydrate = useWorkbenchStore((state) => state.hydrate);
  const selectTask = useWorkbenchStore((state) => state.selectTask);
  const workbenchQuery = useWorkbenchQuery();

  useEffect(() => {
    if (workbenchQuery.data) {
      hydrate(workbenchQuery.data);
    }
  }, [hydrate, workbenchQuery.data]);

  useEffect(() => {
    eventStream.connect();
    return eventStream.subscribe((event) => {
      if (event.type === "onSelectTask") selectTask(event.taskId);
    });
  }, [selectTask]);

  if (workbenchQuery.isLoading) {
    return (
      <AppShell project={project}>
        <div className="grid h-full place-items-center bg-bg-app p-4">
          <EmptyState label="Loading workbench" />
        </div>
      </AppShell>
    );
  }

  if (workbenchQuery.isError) {
    return (
      <AppShell project={project}>
        <div className="grid h-full place-items-center bg-bg-app p-4">
          <EmptyState label="Unable to load workbench state" />
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell project={project}>
      <SplitPane
        left={<div className="grid h-full min-h-0 grid-rows-[minmax(16rem,40%)_minmax(0,1fr)]"><BoardFlow /><TaskWorkbenchMain /></div>}
        right={<TaskRightSidebar />}
        bottom={<TaskBottomPanel />}
      />
    </AppShell>
  );
}
