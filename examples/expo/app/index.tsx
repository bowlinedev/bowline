import { useQuery } from "@tanstack/react-query";
import { Link } from "expo-router";
import { FlatList, Pressable, StyleSheet, Text, View } from "react-native";
import { bq } from "../src/api";

export default function InvoicesScreen() {
  const list = useQuery(bq.invoices.list.queryOptions({ limit: 20 }));
  return (
    <View style={styles.screen}>
      <Link href="/create" asChild>
        <Pressable style={styles.button} testID="new-invoice">
          <Text style={styles.buttonText}>new invoice</Text>
        </Pressable>
      </Link>
      {list.isPending && <Text testID="loading">loading…</Text>}
      {list.error && (
        <Text testID="error" style={styles.error}>
          {list.error.message}
        </Text>
      )}
      <FlatList
        data={list.data?.items ?? []}
        keyExtractor={(invoice) => String(invoice.id)}
        renderItem={({ item }) => (
          <View style={styles.row} testID={`invoice-${item.id}`}>
            <Text style={styles.id}>#{item.id}</Text>
            <Text testID={`status-${item.id}`}>{item.status}</Text>
            <Text style={styles.total}>{item.total}</Text>
          </View>
        )}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, padding: 16, gap: 12 },
  button: { backgroundColor: "#1d4ed8", borderRadius: 8, padding: 12, alignItems: "center" },
  buttonText: { color: "#fff", fontWeight: "600" },
  row: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 10 },
  id: { fontWeight: "600" },
  total: { fontVariant: ["tabular-nums"] },
  error: { color: "#b91c1c" },
});
