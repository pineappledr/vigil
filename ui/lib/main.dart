import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:google_fonts/google_fonts.dart';

void main() {
  runApp(const VigilApp());
}

class VigilApp extends StatelessWidget {
  const VigilApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'Vigil',
      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: const Color(0xFF0F172A), // Dark Navy
        textTheme: GoogleFonts.interTextTheme(
          Theme.of(context).textTheme,
        ).apply(bodyColor: Colors.white, displayColor: Colors.white),
      ),
      home: const DashboardScreen(),
    );
  }
}

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  String status = "Waiting for connection...";
  List<dynamic> history = [];

  @override
  void initState() {
    super.initState();
    fetchHistory();
  }

  Future<void> fetchHistory() async {
    try {
      final response = await http.get(
        Uri.parse('http://localhost:8090/api/history'),
      );

      if (response.statusCode == 200) {
        setState(() {
          status = "Connected to Vigil Server";
          history = json.decode(response.body);
        });
      } else {
        setState(() => status = "Server Error: ${response.statusCode}");
      }
    } catch (e) {
      setState(() => status = "Connection Failed");
    }
  }

  @override
  Widget build(BuildContext context) {
    bool isConnected = status.startsWith("Connected");

    return Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Text(
              "Vigil",
              style: TextStyle(fontSize: 40, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 20),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
              decoration: BoxDecoration(
                color: isConnected
                    ? Colors.green.withValues(alpha: 0.2)
                    : Colors.red.withValues(alpha: 0.2),
                borderRadius: BorderRadius.circular(10),
                border: Border.all(
                  color: isConnected ? Colors.green : Colors.red,
                ),
              ),
              child: Text(status, style: const TextStyle(fontSize: 16)),
            ),
            const SizedBox(height: 20),
            const Text("Recent Reports:", style: TextStyle(color: Colors.grey)),
            const SizedBox(height: 10),
            SizedBox(
              height: 200,
              width: 300,
              child: ListView.builder(
                itemCount: history.length,
                itemBuilder: (context, index) {
                  final item = history[index];
                  return Card(
                    color: Colors.white10,
                    child: ListTile(
                      title: Text(
                        item['hostname'],
                        style: const TextStyle(color: Colors.white),
                      ),
                      subtitle: Text(
                        item['timestamp'],
                        style: const TextStyle(color: Colors.white54),
                      ),
                      leading: const Icon(Icons.storage, color: Colors.blue),
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
