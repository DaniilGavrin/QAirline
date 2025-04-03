import 'package:flutter_bloc/flutter_bloc.dart';
import 'auth_service.dart';
import 'auth_state.dart';

class AuthCubit extends Cubit<AuthState> {
  final AuthService authService;

  AuthCubit(this.authService) : super(AuthInitial());

  Future<void> authenticate({
    required String username,
    required String password,
  }) async {
    emit(AuthLoading());
    try {
      await authService.authenticate(username, password);
      if (authService.authToken != null && authService.channel != null) {
        emit(AuthSuccess(
          token: authService.authToken!,
          channel: authService.channel!,
        ));
      } else {
        emit(AuthError(message: 'Authentication failed'));
      }
    } catch (e) {
      emit(AuthError(message: e.toString()));
    }
  }
}