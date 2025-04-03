import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'screen/auth_screen.dart';
import 'screen/main_screen.dart';
import 'service/auth_service.dart';

void main() {
  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider(
          create: (context) => AuthService(),
        ),
      ],
      child: const MyApp(),
    ),
  );
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'QAirline',
      initialRoute: '/',
      routes: {
        '/': (context) => const AuthScreen(),
      },
      // Убрали маршрут '/main' из routes
    );
  }
}