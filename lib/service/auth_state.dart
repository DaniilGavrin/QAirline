import 'package:web_socket_channel/web_socket_channel.dart';

abstract class AuthState {}

class AuthInitial extends AuthState {}

class AuthLoading extends AuthState {}

class AuthSuccess extends AuthState {
  final String token;
  final WebSocketChannel channel;

  AuthSuccess({required this.token, required this.channel});
}

class AuthError extends AuthState {
  final String message;

  AuthError({required this.message});
}