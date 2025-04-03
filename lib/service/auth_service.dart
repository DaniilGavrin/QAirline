import 'package:flutter/material.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'dart:convert';

class AuthService with ChangeNotifier {
  WebSocketChannel? channel;
  String? authToken;
  static const String _tokenKey = 'auth_token';

  Future<void> authenticate(String username, String password) async {
    try {
      channel = WebSocketChannel.connect(Uri.parse('ws://localhost:8080/ws'));

      final request = {
        'type': 'auth',
        'payload': {'username': username, 'password': password}
      };
      channel!.sink.add(jsonEncode(request));

      final response = await channel!.stream
          .timeout(const Duration(seconds: 30))
          .firstWhere((data) => jsonDecode(data)['type'] == 'auth_response');

      authToken = jsonDecode(response)['payload']['token'];
      await _saveToken(authToken!);
      notifyListeners();
    } catch (e) {
      rethrow;
    }
  }

  Future<void> _saveToken(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
  }

  Future<void> logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    authToken = null;
    channel?.sink.close();
    notifyListeners();
  }
}