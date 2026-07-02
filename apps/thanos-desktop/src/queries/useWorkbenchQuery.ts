import { useQuery } from "@tanstack/react-query";
import { NativeBackend } from "../services/nativeBackend";

const backend = new NativeBackend();

export function useWorkbenchQuery() {
  return useQuery({
    queryKey: ["workbench"],
    queryFn: async () => {
      const snapshot = await backend.loadWorkbenchState();
      if (!snapshot) {
        throw new Error("Unable to load Thanos workbench state");
      }
      return snapshot;
    },
    staleTime: 5_000,
  });
}
