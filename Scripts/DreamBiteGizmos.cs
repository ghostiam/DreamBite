#if UNITY_EDITOR

using System;
using System.Collections.Generic;
using Unity.Collections;
using UnityEditor;
using UnityEngine;
using VRC.Dynamics;
using VRC.SDK3.Avatars.Components;
using VRC.SDK3.Dynamics.Constraint.Components;
using VRC.SDKBase;

namespace GhostIAm.DreamBite
{
    [AddComponentMenu("GhostIAm/DreamBiteGizmos")]
    public class DreamBiteGizmos : MonoBehaviour, IEditorOnly
    {
        public Color color = new Color(1, 1, 1, 1);
        public bool preservingSize;

        [Header("Capsule")] public bool drawCapsule = true;
        public float radius = 0.1f;
        public float length = 0.5f;

        [Header("Arrow")] public bool drawArrow = true;
        public Vector3 arrowStart = Vector3.zero;
        public Vector3 arrowEnd = Vector3.forward;
        public float arrowCapSize = 0.2f;

        [Header("Offsets")] public Vector3 position = Vector3.zero;
        public Quaternion rotation = Quaternion.identity;

        void OnDrawGizmosSelected()
        {
            var scaledRadius = radius;
            var scaledLength = length;
            var scaledArrowStart = arrowStart;
            var scaledArrowEnd = arrowEnd;
            var scaledArrowCapSize = arrowCapSize;
            if (!preservingSize)
            {
                var scale = VRCAvatarDescriptor.MaxScale(transform.lossyScale);
                scaledRadius *= scale;
                scaledLength *= scale;
                scaledArrowStart *= scale;
                scaledArrowEnd *= scale;
                scaledArrowCapSize *= scale;
            }

            var pos = transform.position + position;
            var rot = transform.rotation * rotation;
            if (drawCapsule) DrawCapsule(pos, rot, scaledLength, scaledRadius, color);
            if (drawArrow)
                DrawArrow(
                    pos + rot * scaledArrowStart,
                    pos + rot * scaledArrowEnd,
                    color,
                    scaledArrowCapSize
                );
        }

        public static void DrawCapsule(
            Vector3 worldPos,
            Quaternion worldRot,
            float worldLength,
            float worldRadius,
            Color color
        )
        {
            Handles.color = color;
            HandlesUtil.DrawWireCapsule(worldPos, worldRot, worldLength, worldRadius);
        }

        public static void DrawLine(
            Vector3 worldStart,
            Vector3 worldEnd,
            Color color
        )
        {
            Handles.color = color;
            Handles.DrawLine(worldStart, worldEnd);
        }

        public static void DrawArrow(
            Vector3 worldStart,
            Vector3 worldEnd,
            Color color,
            float capSize
        )
        {
            Handles.color = color;
            var dir = worldEnd - worldStart;
            var length = dir.magnitude;
            if (length < capSize || length < 0.01f)
            {
                return;
            }

            DrawLine(worldStart, worldEnd, color);
            Handles.ConeHandleCap(0, worldEnd - dir.normalized * capSize * 0.7f, Quaternion.LookRotation(dir), capSize,
                EventType.Repaint);
        }
    }
}

#endif