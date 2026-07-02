import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";

export function SplitPane({ left, right, bottom }: { left: React.ReactNode; right: React.ReactNode; bottom: React.ReactNode }) {
  return (
    <PanelGroup direction="horizontal" className="min-h-0">
      <Panel defaultSize={76} minSize={55}>
        <PanelGroup direction="vertical">
          <Panel defaultSize={74} minSize={45}>
            {left}
          </Panel>
          <PanelResizeHandle className="h-1 bg-slate-800 hover:bg-purple-primary" />
          <Panel defaultSize={26} minSize={18}>
            {bottom}
          </Panel>
        </PanelGroup>
      </Panel>
      <PanelResizeHandle className="w-1 bg-slate-800 hover:bg-purple-primary" />
      <Panel defaultSize={24} minSize={18}>
        {right}
      </Panel>
    </PanelGroup>
  );
}
