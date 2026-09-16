import { BowlineError } from "@bowlinedev/client";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { bq } from "../src/api";

export default function CreateScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState("1");
  const create = useMutation({
    ...bq.invoices.create.mutationOptions(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
      router.back();
    },
  });
  const issues = create.error instanceof BowlineError ? create.error.issues : [];
  return (
    <View style={styles.screen}>
      <TextInput
        accessibilityLabel="description"
        testID="description"
        placeholder="description"
        value={description}
        onChangeText={setDescription}
        style={styles.input}
      />
      <TextInput
        accessibilityLabel="quantity"
        testID="quantity"
        placeholder="quantity"
        keyboardType="number-pad"
        value={quantity}
        onChangeText={setQuantity}
        style={styles.input}
      />
      <Pressable
        style={styles.button}
        testID="create-invoice"
        disabled={create.isPending}
        onPress={() =>
          create.mutate({
            customerId: 1,
            lines: [{ description, quantity: Number(quantity), unitPrice: "USD 10.00" }],
          })
        }
      >
        <Text style={styles.buttonText}>create invoice</Text>
      </Pressable>
      {issues.length > 0 && (
        <View testID="issues">
          {issues.map((issue) => (
            <Text key={issue.path.join(".")} style={styles.error}>
              {issue.path.join(".")}: {issue.message}
            </Text>
          ))}
        </View>
      )}
      {create.error && issues.length === 0 && (
        <Text testID="error" style={styles.error}>
          {create.error.message}
        </Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, padding: 16, gap: 12 },
  input: { borderWidth: 1, borderColor: "#cbd5e1", borderRadius: 8, padding: 12 },
  button: { backgroundColor: "#1d4ed8", borderRadius: 8, padding: 12, alignItems: "center" },
  buttonText: { color: "#fff", fontWeight: "600" },
  error: { color: "#b91c1c" },
});
