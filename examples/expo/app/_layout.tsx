import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useState } from "react";

export default function Layout() {
  const [queryClient] = useState(() => new QueryClient());
  return (
    <QueryClientProvider client={queryClient}>
      <StatusBar style="auto" />
      <Stack>
        <Stack.Screen name="index" options={{ title: "Invoices" }} />
        <Stack.Screen name="create" options={{ title: "New invoice", presentation: "modal" }} />
      </Stack>
    </QueryClientProvider>
  );
}
